package avatarconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type File struct {
	DefaultAvatarID string   `json:"defaultAvatarId"`
	Avatars         []Avatar `json:"avatars"`
}

type Avatar struct {
	ID                        string   `json:"id"`
	DisplayName               string   `json:"displayName"`
	Enabled                   bool     `json:"enabled"`
	IsDefault                 bool     `json:"isDefault"`
	AvatarModel               string   `json:"avatarModel"`
	Model                     string   `json:"model,omitempty"`
	Voice                     string   `json:"voice,omitempty"`
	Temperature               *float32 `json:"temperature,omitempty"`
	EnableInputTranscription  *bool    `json:"enableInputTranscription,omitempty"`
	EnableOutputTranscription *bool    `json:"enableOutputTranscription,omitempty"`
	QuerySource               string   `json:"querySource"`
	Endpoint                  string   `json:"endpoint"`
	SystemInstruction         string   `json:"systemInstruction"`
	SpecialRules              []string `json:"specialRules"`
	MCPToolName               string   `json:"mcpToolName,omitempty"`
	MCPToolArgument           string   `json:"mcpToolArgument,omitempty"`
}

type Store struct {
	path          string
	defaultAvatar Avatar
	avatars       map[string]Avatar
	ordered       []Avatar
}

func Load(path string) (*Store, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read avatar config: %w", err)
	}

	var cfg File
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse avatar config: %w", err)
	}

	if len(cfg.Avatars) == 0 {
		return nil, errors.New("avatar config has no avatars")
	}

	avatars := make(map[string]Avatar, len(cfg.Avatars))
	ordered := make([]Avatar, 0, len(cfg.Avatars))
	var defaultAvatar Avatar

	defaultID := strings.TrimSpace(cfg.DefaultAvatarID)
	for _, avatar := range cfg.Avatars {
		avatar.ID = strings.TrimSpace(avatar.ID)
		avatar.DisplayName = strings.TrimSpace(avatar.DisplayName)
		avatar.QuerySource = normalizeQuerySource(avatar.QuerySource)
		avatar.Model = strings.TrimSpace(avatar.Model)
		avatar.Voice = strings.TrimSpace(avatar.Voice)
		avatar.Endpoint = strings.TrimSpace(avatar.Endpoint)
		avatar.SystemInstruction = strings.TrimSpace(avatar.SystemInstruction)
		avatar.MCPToolName = strings.TrimSpace(avatar.MCPToolName)
		avatar.MCPToolArgument = strings.TrimSpace(avatar.MCPToolArgument)
		if avatar.ID == "" {
			return nil, errors.New("avatar id is required")
		}
		if avatar.AvatarModel == "" {
			return nil, fmt.Errorf("avatarModel is required for %s", avatar.ID)
		}
		if !strings.HasPrefix(avatar.AvatarModel, "/") {
			avatar.AvatarModel = "/" + strings.TrimLeft(filepath.ToSlash(avatar.AvatarModel), "/")
		}
		if avatar.QuerySource == "" {
			avatar.QuerySource = "llm"
		}
		if _, exists := avatars[avatar.ID]; exists {
			return nil, fmt.Errorf("duplicate avatar id: %s", avatar.ID)
		}
		avatars[avatar.ID] = avatar
		ordered = append(ordered, avatar)
		if avatar.IsDefault || (defaultID != "" && avatar.ID == defaultID) {
			defaultAvatar = avatar
		}
	}

	if defaultAvatar.ID == "" {
		firstEnabled, ok := firstEnabledAvatar(ordered)
		if !ok {
			return nil, errors.New("no enabled avatar found")
		}
		defaultAvatar = firstEnabled
	}

	return &Store{
		path:          path,
		defaultAvatar: defaultAvatar,
		avatars:       avatars,
		ordered:       ordered,
	}, nil
}

func (s *Store) Default() Avatar {
	return s.defaultAvatar
}

func (s *Store) Get(id string) (Avatar, bool) {
	avatar, ok := s.avatars[strings.TrimSpace(id)]
	return avatar, ok
}

func (s *Store) Enabled() []Avatar {
	out := make([]Avatar, 0, len(s.ordered))
	for _, avatar := range s.ordered {
		if avatar.Enabled {
			out = append(out, avatar)
		}
	}
	return out
}

func (s *Store) Path() string {
	return s.path
}

func normalizeQuerySource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "mcp":
		return "mcp"
	case "llm":
		return "llm"
	default:
		return strings.ToLower(strings.TrimSpace(source))
	}
}

func firstEnabledAvatar(avatars []Avatar) (Avatar, bool) {
	for _, avatar := range avatars {
		if avatar.Enabled {
			return avatar, true
		}
	}
	return Avatar{}, false
}
