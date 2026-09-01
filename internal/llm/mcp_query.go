package llm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/asccclass/sherryserver/internal/mcp"
	mcptypes "github.com/mark3labs/mcp-go/mcp"
)

type MCPQueryConfig struct {
	Endpoint               string
	ToolName               string
	ToolArgument           string
	SystemPrompt           string
	EnableSensitiveLookup  bool
	EnableSensitiveGuard   bool
}

type MCPQueryClient struct {
	cfg MCPQueryConfig
}

func NewMCPQueryClient(cfg MCPQueryConfig) *MCPQueryClient {
	return &MCPQueryClient{cfg: cfg}
}

func (c *MCPQueryClient) Generate(ctx context.Context, prompt Prompt) (Reply, error) {
	if strings.TrimSpace(c.cfg.Endpoint) == "" {
		return Reply{}, fmt.Errorf("mcp endpoint is empty")
	}

	client, err := mcp.NewSSEClient(c.cfg.Endpoint)
	if err != nil {
		return Reply{}, err
	}
	defer client.Close()

	toolsRes, err := client.ListTools(ctx)
	if err != nil {
		return Reply{}, err
	}
	if toolsRes == nil || len(toolsRes.Tools) == 0 {
		return Reply{}, fmt.Errorf("mcp server returned no tools")
	}

	tool, err := chooseMCPTool(toolsRes.Tools, c.cfg.ToolName)
	if err != nil {
		return Reply{}, err
	}

	args := buildMCPArguments(tool, prompt.User, c.cfg.ToolArgument)
	res, err := client.CallTool(ctx, tool.Name, args)
	if err != nil {
		return Reply{}, err
	}

	text := strings.TrimSpace(sanitizeMCPResultText(resultText(res)))
	if text == "" {
		return Reply{}, fmt.Errorf("mcp tool returned empty result")
	}
	if fallback, ok := querySensitiveKnowledge(prompt.User, text, c.cfg); ok {
		return Reply{Text: fallback}, nil
	}

	return Reply{Text: text}, nil
}

func chooseMCPTool(tools []mcptypes.Tool, requested string) (mcptypes.Tool, error) {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		for _, tool := range tools {
			if tool.Name == requested {
				return tool, nil
			}
		}
		return mcptypes.Tool{}, fmt.Errorf("mcp tool not found: %s", requested)
	}
	if len(tools) == 1 {
		return tools[0], nil
	}
	return mcptypes.Tool{}, fmt.Errorf("multiple mcp tools available; set mcpToolName in avatars.json")
}

func buildMCPArguments(tool mcptypes.Tool, userText string, explicitArg string) map[string]any {
	argName := strings.TrimSpace(explicitArg)
	if argName == "" {
		for _, candidate := range []string{"query", "question", "text", "input", "message", "prompt"} {
			if hasSchemaProperty(tool, candidate) {
				argName = candidate
				break
			}
		}
	}
	if argName == "" {
		return map[string]any{}
	}
	return map[string]any{
		argName: buildFocusedQuestion(userText),
	}
}

func buildFocusedQuestion(userText string) string {
	trimmed := strings.TrimSpace(userText)
	if trimmed == "" {
		return ""
	}

	topic := extractTopic(trimmed)
	if topic != "" {
		return topic
	}
	return trimmed
}

func extractKeywords(input string) []string {
	fields := strings.FieldsFunc(input, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})

	stopwords := map[string]struct{}{
		"幫我": {}, "請": {}, "找": {}, "查": {}, "查詢": {}, "搜尋": {}, "一下": {},
		"網址": {}, "帳密": {}, "帳號": {}, "密碼": {}, "連結": {}, "網站": {},
		"的": {}, "及": {}, "和": {}, "與": {},
	}

	seen := make(map[string]struct{})
	keywords := make([]string, 0, len(fields))
	for _, field := range fields {
		term := strings.TrimSpace(field)
		if term == "" {
			continue
		}
		if _, skip := stopwords[term]; skip {
			continue
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		keywords = append(keywords, term)
	}
	return keywords
}

var urlPattern = regexp.MustCompile(`https?://[^\s<>()]+`)
var sourceTrailPattern = regexp.MustCompile("(?i),?\\s*sourced from\\s*`?\\[\\[[^\\]]+\\]\\]`?\\.?$")
var windowsPathLinePattern = regexp.MustCompile("(?i)^`?[a-z]:\\\\[^`\\n]+`?(?:,?\\s*sourced from\\s*`?\\[\\[[^\\]]+\\]\\]`?)?\\.?$")

func querySensitiveKnowledge(userText string, mcpText string, cfg MCPQueryConfig) (string, bool) {
	if !cfg.EnableSensitiveLookup {
		return "", false
	}

	if cfg.EnableSensitiveGuard && !asksForSensitiveLookup(userText) {
		return "", false
	}

	topic := extractTopic(userText)
	if topic == "" || looksRelevant(topic, mcpText) {
		return "", false
	}

	root := resolveBrainRoot()
	if root == "" {
		return "", false
	}

	hits := make([]string, 0, 8)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".txt" {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		text := string(content)
		if !strings.Contains(text, topic) {
			return nil
		}

		urls := uniqueStrings(urlPattern.FindAllString(text, -1))
		if len(urls) == 0 {
			return nil
		}

		if asksForCredentials(userText) {
			account, password := extractCredentialFields(text)
			if account != "" || password != "" {
				hit := fmt.Sprintf("根據本機知識庫 `%s`：\n- 網址：%s", path, strings.Join(urls, "、"))
				if account != "" {
					hit += "\n- 帳號：" + account
				}
				if password != "" {
					hit += "\n- 密碼：" + password
				}
				hits = append(hits, hit)
				return nil
			}
		}

		hits = append(hits, fmt.Sprintf("根據本機知識庫 `%s`：\n- 網址：%s", path, strings.Join(urls, "、")))
		return nil
	})
	if err != nil || len(hits) == 0 {
		return "", false
	}

	sort.SliceStable(hits, func(i, j int) bool {
		return len(hits[i]) < len(hits[j])
	})
	return hits[0], true
}

func resolveBrainRoot() string {
	candidates := []string{
		os.Getenv("MYBRAIN_ROOT"),
		os.Getenv("MYBRAIN_DATA_ROOT"),
		`D:\myprograms\mybraindata`,
		filepath.Clean(filepath.Join("..", "mybraindata")),
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

func extractTopic(userText string) string {
	replacer := strings.NewReplacer(
		"幫我找", "",
		"請幫我找", "",
		"請找", "",
		"幫我查", "",
		"請幫我查", "",
		"請查", "",
		"網址及帳密", "",
		"網址和帳密", "",
		"網址與帳密", "",
		"帳號密碼", "",
		"帳號", "",
		"密碼", "",
		"網址", "",
		"連結", "",
		"網站", "",
		"相關資訊", "",
		"相關資料", "",
		"資訊", "",
		"資料", "",
		"的", "",
	)
	topic := strings.TrimSpace(replacer.Replace(userText))
	return strings.Trim(topic, "：:，,。!?！？ ")
}

func looksRelevant(topic string, text string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(topic))
}

func asksForCredentials(userText string) bool {
	return strings.Contains(userText, "帳密") ||
		strings.Contains(userText, "帳號") ||
		strings.Contains(userText, "密碼")
}

func asksForSensitiveLookup(userText string) bool {
	return asksForCredentials(userText) ||
		strings.Contains(userText, "網址") ||
		strings.Contains(userText, "連結") ||
		strings.Contains(userText, "網站")
}

func extractCredentialFields(text string) (string, string) {
	var account string
	var password string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case account == "" && (strings.Contains(trimmed, "帳號") || strings.Contains(trimmed, "account")):
			account = strings.TrimSpace(strings.TrimLeft(strings.SplitN(trimmed, "：", 2)[safeIndex(strings.SplitN(trimmed, "：", 2), 1)], ":"))
		case password == "" && (strings.Contains(trimmed, "密碼") || strings.Contains(strings.ToLower(trimmed), "password")):
			password = strings.TrimSpace(strings.TrimLeft(strings.SplitN(trimmed, "：", 2)[safeIndex(strings.SplitN(trimmed, "：", 2), 1)], ":"))
		}
	}
	return account, password
}

func safeIndex(values []string, idx int) int {
	if idx < len(values) {
		return idx
	}
	return 0
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func hasSchemaProperty(tool mcptypes.Tool, key string) bool {
	if tool.InputSchema.Properties == nil {
		return false
	}
	_, ok := tool.InputSchema.Properties[key]
	return ok
}

func resultText(res *mcptypes.CallToolResult) string {
	if res == nil {
		return ""
	}
	var parts []string
	for _, c := range res.Content {
		switch value := c.(type) {
		case mcptypes.TextContent:
			if strings.TrimSpace(value.Text) != "" {
				parts = append(parts, strings.TrimSpace(value.Text))
			}
		default:
			if rendered := fmt.Sprintf("%v", value); strings.TrimSpace(rendered) != "" {
				parts = append(parts, strings.TrimSpace(rendered))
			}
		}
	}
	return strings.Join(parts, "\n")
}

func sanitizeMCPResultText(input string) string {
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	cleaned := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(cleaned) > 0 && cleaned[len(cleaned)-1] != "" {
				cleaned = append(cleaned, "")
			}
			continue
		}
		if windowsPathLinePattern.MatchString(trimmed) {
			continue
		}

		trimmed = strings.TrimSpace(sourceTrailPattern.ReplaceAllString(trimmed, ""))
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}

	for len(cleaned) > 0 && cleaned[len(cleaned)-1] == "" {
		cleaned = cleaned[:len(cleaned)-1]
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
