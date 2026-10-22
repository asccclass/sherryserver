package splitter

import "strings"

func SplitSentences(text string) []string {
	var sentences []string
	var builder strings.Builder

	flush := func() {
		s := strings.TrimSpace(builder.String())
		if s != "" {
			sentences = append(sentences, s)
		}
		builder.Reset()
	}

	for _, r := range text {
		switch r {
		case '.', '?', '!', '。', '？', '！', '\n', '\r':
			flush()
		default:
			builder.WriteRune(r)
		}
	}
	flush()
	return sentences
}
