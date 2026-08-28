package llm

func MergeSystemPrompts(base string, session string) string {
	if base == "" {
		return session
	}
	if session == "" {
		return base
	}
	return base + "\n\n" + session
}
