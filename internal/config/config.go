package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	ListenAddr string
	Web        WebConfig
	Gemini     GeminiConfig
	LLM        LLMConfig
	TTS        TTSConfig
	LiveTalking LiveTalkingConfig
	LiveKit    LiveKitConfig
	MCP        MCPConfig
}

type WebConfig struct {
	StaticPath string
	IndexPath  string
}

type GeminiConfig struct {
	APIKey     string
	APIVersion string
	Model      string
}

type LLMConfig struct {
	APIURL       string
	APIKey       string
	Model        string
	SystemPrompt string
	ProcessingMessage string
	ErrorMessage      string
	Timeout      time.Duration
}

type TTSConfig struct {
	Provider string
	Voice    string
	Format   string
	BaseURL  string
	APIKey   string
}

type LiveTalkingConfig struct {
	BaseURL   string
	APIKey    string
	AvatarID  string
	Transport string
}

type LiveKitConfig struct {
	URL             string
	APIKey          string
	APISecret       string
	DefaultRoom     string
	IngressBaseURL  string
}

type MCPConfig struct {
	SSEURL                 string
	EnableSensitiveLookup  bool
	EnableSensitiveGuard   bool
}

func LoadFromEnv() (*Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	defaultStaticPath := resolveStaticPath(wd)
	indexPath := firstNonEmpty(
		strings.TrimSpace(os.Getenv("WEB_INDEX_PATH")),
		strings.TrimSpace(os.Getenv("INDEX_PATH")),
		"index.html",
	)

	listenAddr := strings.TrimSpace(os.Getenv("LISTEN_ADDR"))
	if listenAddr == "" {
		listenAddr = ":8000"
	}

	apiVersion := strings.TrimSpace(os.Getenv("GEMINI_API_VERSION"))
	if apiVersion == "" {
		apiVersion = "v1beta"
	}

	llmURL := strings.TrimSpace(os.Getenv("CUSTOM_LLM_API_URL"))
	llmKey := strings.TrimSpace(os.Getenv("CUSTOM_LLM_API_KEY"))
	llmModel := strings.TrimSpace(os.Getenv("CUSTOM_LLM_MODEL"))
	if llmURL == "" && strings.TrimSpace(os.Getenv("OLLAMA_URL")) != "" {
		llmURL = strings.TrimSpace(os.Getenv("OLLAMA_URL"))
		llmModel = firstNonEmpty(llmModel, strings.TrimSpace(os.Getenv("OLLAMA_MODEL")))
	}
	if llmURL == "" && strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != "" {
		llmURL = "https://api.openai.com/v1/chat/completions"
		llmKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		if llmModel == "" {
			llmModel = "gpt-4.1-mini"
		}
	}

	cfg := &Config{
		ListenAddr: listenAddr,
		Web: WebConfig{
			StaticPath: defaultStaticPath,
			IndexPath:  indexPath,
		},
		Gemini: GeminiConfig{
			APIKey:     strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
			APIVersion: apiVersion,
			Model:      strings.TrimSpace(os.Getenv("GEMINI_MODEL")),
		},
		LLM: LLMConfig{
			APIURL:            normalizeChatCompletionsURL(llmURL),
			APIKey:            llmKey,
			Model:             llmModel,
			SystemPrompt:      strings.TrimSpace(os.Getenv("CUSTOM_LLM_SYSTEM_PROMPT")),
			ProcessingMessage: firstNonEmpty(strings.TrimSpace(os.Getenv("CUSTOM_LLM_PROCESSING_MESSAGE")), "請稍候，我正在處理您的需求。"),
			ErrorMessage:      firstNonEmpty(strings.TrimSpace(os.Getenv("CUSTOM_LLM_ERROR_MESSAGE")), "抱歉，我剛剛處理失敗了，請再說一次。"),
			Timeout:           mustParseDuration(firstNonEmpty(strings.TrimSpace(os.Getenv("CUSTOM_LLM_TIMEOUT")), "20s")),
		},
		TTS: TTSConfig{
			Provider: strings.TrimSpace(os.Getenv("TTS_PROVIDER")),
			Voice:    strings.TrimSpace(os.Getenv("TTS_VOICE")),
			Format:   firstNonEmpty(strings.TrimSpace(os.Getenv("TTS_FORMAT")), "wav"),
			BaseURL:  strings.TrimSpace(os.Getenv("TTS_BASE_URL")),
			APIKey:   strings.TrimSpace(os.Getenv("TTS_API_KEY")),
		},
		LiveTalking: LiveTalkingConfig{
			BaseURL:   strings.TrimSpace(os.Getenv("LIVETALKING_BASE_URL")),
			APIKey:    strings.TrimSpace(os.Getenv("LIVETALKING_API_KEY")),
			AvatarID:  strings.TrimSpace(os.Getenv("LIVETALKING_AVATAR_ID")),
			Transport: firstNonEmpty(strings.TrimSpace(os.Getenv("LIVETALKING_TRANSPORT")), "livekit_ingress"),
		},
		LiveKit: LiveKitConfig{
			URL:            strings.TrimSpace(os.Getenv("LIVEKIT_URL")),
			APIKey:         strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")),
			APISecret:      strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET")),
			DefaultRoom:    strings.TrimSpace(os.Getenv("LIVEKIT_DEFAULT_ROOM")),
			IngressBaseURL: strings.TrimSpace(os.Getenv("LIVEKIT_INGRESS_BASE_URL")),
		},
		MCP: MCPConfig{
			SSEURL:                strings.TrimSpace(os.Getenv("MCP_SSE_URL")),
			EnableSensitiveLookup: parseBoolEnv(true, "ENABLE_SENSITIVE_LOOKUP", "MCP_ENABLE_SENSITIVE_LOOKUP"),
			EnableSensitiveGuard:  parseBoolEnv(true, "ENABLE_SENSITIVE_GUARD", "MCP_ENABLE_SENSITIVE_GUARD"),
		},
	}

	if cfg.Gemini.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required")
	}

	return cfg, nil
}

func resolveStaticPath(wd string) string {
	configured := firstNonEmpty(
		strings.TrimSpace(os.Getenv("WEB_STATIC_PATH")),
		strings.TrimSpace(os.Getenv("DOCUMENT_ROOT")),
		strings.TrimSpace(os.Getenv("DocumentRoot")),
	)
	if configured != "" {
		if filepath.IsAbs(configured) {
			return configured
		}
		return filepath.Join(wd, configured)
	}

	candidates := []string{
		filepath.Join(wd, "www", "html"),
		filepath.Join(filepath.Dir(filepath.Dir(wd)), "www", "html"),
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	return filepath.Join(wd, "www", "html")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeChatCompletionsURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	switch {
	case strings.HasSuffix(parsed.Path, "/chat/completions"):
		return parsed.String()
	case strings.HasSuffix(parsed.Path, "/v1"):
		parsed.Path = parsed.Path + "/chat/completions"
	case parsed.Path == "":
		parsed.Path = "/v1/chat/completions"
	default:
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/chat/completions"
	}

	return parsed.String()
}

func mustParseDuration(raw string) time.Duration {
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 20 * time.Second
	}
	return duration
}

func parseBoolEnv(fallback bool, keys ...string) bool {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}
		switch strings.ToLower(value) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return fallback
}
