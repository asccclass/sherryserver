// Assistant text visibility rules.
// Keep this file focused on adjustable patterns so the UI logic can stay simple.
export const ASSISTANT_TEXT_FILTER_RULES = {
  // Any match here is always allowed through, even if blacklist rules also match.
  // Add patterns for languages or phrases we always want the user to see.
  whitelistPatterns: [
    /[一-龥ㄅ-ㄩ]/,
  ],
  // Content keywords commonly seen in system-like execution notes rather than user-facing replies.
  // Add terms here when a new hidden internal phrase appears repeatedly.
  blacklistKeywordPatterns: [
    /directive/i,
    /provided text/i,
    /no further thoughts/i,
    /without modification/i,
    /executing this process/i,
    /generating audio/i,
  ],
  // First-person process narration often signals internal reasoning or tool-execution chatter.
  // Keep these broad enough to catch variants, but avoid phrases common in real answers.
  blacklistTonePatterns: [
    /i'm now/i,
    /i have\b/i,
    /i've\b/i,
    /my task/i,
    /primary job/i,
    /following the instruction/i,
    /followed the directive/i,
    /precisely as defined/i,
  ],
  structuralPatterns: {
    // Markdown-style English headings are common in hidden internal notes.
    headingLike: /^#{1,6}\s*[a-z]/i,
    // Used as one signal that the whole block is structured English-only content.
    strongEnglishOnly: /^[\s\n\r\p{L}\p{N}\p{P}\p{S}]+$/u,
  },
  // Only treat long English-only blocks as suspicious; short snippets are too noisy.
  minStructuredEnglishLength: 80,
  // Number of blacklist signals required before we hide a message.
  // Raise this to be safer, lower it to filter more aggressively.
  minBlacklistScore: 2,
};
