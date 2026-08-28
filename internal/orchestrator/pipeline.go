package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/asccclass/sherryserver/internal/llm"
	"github.com/asccclass/sherryserver/internal/session"
	"github.com/asccclass/sherryserver/internal/stt"
	"google.golang.org/genai"
)

type StatusWriter interface {
	SendStatus(message string)
	SendPrompt(systemPrompt string, userPrompt string)
	SendAssistantText(text string)
}

type Service struct {
	SessionID         string
	Sessions          *session.Manager
	Setup             *stt.SessionSetup
	Session           *genai.Session
	Writer            StatusWriter
	LLM               llm.Client
	ProcessingMessage string
	ErrorMessage      string

	mu              sync.Mutex
	inputTranscript string
	activeCancel    context.CancelFunc
	activeRequestID uint64
	activeSource    string
}

func (s *Service) HandleLiveServerMessage(message *genai.LiveServerMessage) {
	if message == nil || message.ServerContent == nil {
		return
	}

	content := message.ServerContent
	if content.InterimInputTranscription != nil && strings.TrimSpace(content.InterimInputTranscription.Text) != "" {
		s.CancelActiveRequest("New speech detected. Previous task canceled.")
	}
	if content.Interrupted {
		s.mu.Lock()
		s.inputTranscript = ""
		s.mu.Unlock()
		s.updateSession(func(current *session.Session) {
			current.State = session.StateInterrupted
			current.RecognizedText = ""
		})
	}

	if content.InputTranscription != nil {
		s.handleInputTranscription(content.InputTranscription.Text, content.InputTranscription.Finished)
	}
}

func (s *Service) StartTextTurn(userText string) {
	s.updateSession(func(current *session.Session) {
		current.State = session.StateThinking
		current.RecognizedText = strings.TrimSpace(userText)
		current.SystemPrompt = s.Setup.SystemInstruction
		current.LastError = ""
	})
	s.startLLMFlow(strings.TrimSpace(userText), false)
}

func (s *Service) handleInputTranscription(text string, finished bool) {
	trimmed := strings.TrimSpace(text)

	s.mu.Lock()
	if trimmed != "" {
		s.inputTranscript = mergeProgressiveText(s.inputTranscript, trimmed)
	}
	finalText := strings.TrimSpace(s.inputTranscript)
	if finished {
		s.inputTranscript = ""
	}
	s.mu.Unlock()

	if finished && finalText != "" {
		s.updateSession(func(current *session.Session) {
			current.State = session.StateThinking
			current.RecognizedText = finalText
			current.SystemPrompt = s.Setup.SystemInstruction
			current.LastError = ""
		})
		s.startLLMFlow(finalText, true)
	}
}

func (s *Service) startLLMFlow(userText string, fromSpeech bool) {
	s.CancelActiveRequest("A newer request replaced the previous task.")

	requestID := atomic.AddUint64(&s.activeRequestID, 1)
	ctx, cancel := context.WithCancel(context.Background())

	s.mu.Lock()
	s.activeCancel = cancel
	s.activeSource = sourceLabel(fromSpeech)
	s.mu.Unlock()

	go func() {
		defer s.releaseActiveRequest(requestID, cancel)

		s.Writer.SendStatus(fmt.Sprintf("User input recognized from %s: %s", sourceLabel(fromSpeech), userText))
		s.updateSession(func(current *session.Session) {
			current.State = session.StateThinking
			current.RecognizedText = userText
			current.SystemPrompt = s.Setup.SystemInstruction
			current.LastError = ""
		})
		if s.isSuperseded(requestID) || ctx.Err() != nil {
			return
		}

		s.Writer.SendStatus("Speaking processing message...")
		s.updateSession(func(current *session.Session) {
			current.State = session.StateTTS
			current.LLMReply = s.ProcessingMessage
		})
		if err := s.speakText(s.ProcessingMessage); err != nil {
			s.Writer.SendStatus("Failed to speak processing message: " + err.Error())
			s.updateSession(func(current *session.Session) {
				current.State = session.StateError
				current.LastError = err.Error()
			})
		}
		if s.isSuperseded(requestID) || ctx.Err() != nil {
			return
		}

		s.Writer.SendStatus("Calling custom LLM...")
		s.updateSession(func(current *session.Session) {
			current.State = session.StateThinking
		})
		reply, err := s.LLM.Generate(ctx, llm.Prompt{
			System: s.Setup.SystemInstruction,
			User:   userText,
		})
		s.Writer.SendPrompt(strings.TrimSpace(s.Setup.SystemInstruction), userText)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil || s.isSuperseded(requestID) {
				s.Writer.SendStatus("Previous LLM task canceled.")
				s.updateSession(func(current *session.Session) {
					current.State = session.StateInterrupted
					current.LastError = "canceled"
				})
				return
			}
			s.Writer.SendStatus("Custom LLM error: " + err.Error())
			reply.Text = s.ErrorMessage
			s.updateSession(func(current *session.Session) {
				current.State = session.StateError
				current.LastError = err.Error()
				current.LLMReply = reply.Text
			})
		} else {
			s.Writer.SendStatus("Custom LLM response ready.")
			s.Writer.SendAssistantText(reply.Text)
			s.updateSession(func(current *session.Session) {
				current.State = session.StateTTS
				current.LLMReply = reply.Text
				current.LastError = ""
			})
		}

		if s.isSuperseded(requestID) || ctx.Err() != nil {
			return
		}

		s.Writer.SendStatus("Speaking final response...")
		s.updateSession(func(current *session.Session) {
			current.State = session.StateStreaming
			current.LLMReply = reply.Text
		})
		if err := s.speakText(reply.Text); err != nil {
			s.Writer.SendStatus("Failed to speak final response: " + err.Error())
			s.updateSession(func(current *session.Session) {
				current.State = session.StateError
				current.LastError = err.Error()
			})
		}
	}()
}

func (s *Service) speakText(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	return s.Session.SendClientContent(genai.LiveClientContentInput{
		Turns: []*genai.Content{
			genai.NewContentFromText("SPEAK_EXACTLY: "+text, genai.RoleUser),
		},
	})
}

func (s *Service) CancelActiveRequest(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeCancel != nil {
		s.activeCancel()
		s.activeCancel = nil
		if strings.TrimSpace(reason) != "" {
			s.Writer.SendStatus(reason)
		}
		s.updateSession(func(current *session.Session) {
			current.State = session.StateInterrupted
			current.LastError = reason
		})
	}
	s.activeSource = ""
}

func (s *Service) releaseActiveRequest(requestID uint64, cancel context.CancelFunc) {
	cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if atomic.LoadUint64(&s.activeRequestID) == requestID && s.activeCancel != nil {
		s.activeCancel = nil
		s.activeSource = ""
	}
}

func (s *Service) isSuperseded(requestID uint64) bool {
	return atomic.LoadUint64(&s.activeRequestID) != requestID
}

func mergeProgressiveText(current, incoming string) string {
	current = strings.TrimSpace(current)
	incoming = strings.TrimSpace(incoming)

	switch {
	case current == "":
		return incoming
	case incoming == "":
		return current
	case strings.HasPrefix(incoming, current):
		return incoming
	case strings.HasPrefix(current, incoming):
		return current
	case strings.Contains(current, incoming):
		return current
	default:
		return current + incoming
	}
}

func sourceLabel(fromSpeech bool) string {
	if fromSpeech {
		return "speech"
	}
	return "text"
}

func (s *Service) updateSession(mutate func(*session.Session)) {
	if s.Sessions == nil || strings.TrimSpace(s.SessionID) == "" {
		return
	}
	s.Sessions.Update(s.SessionID, mutate)
}
