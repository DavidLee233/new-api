package constant

import "testing"

func TestCanonicalClaudeModelAliasUsesFullClaudePrefix(t *testing.T) {
	tests := map[string]string{
		"sonnet-4-6":           "claude-sonnet-4-6",
		"sonnet-4.6":           "claude-sonnet-4-6",
		"sonnet-4-6-thinking":  "claude-sonnet-4-6-thinking",
		"opus-4-6":             "claude-opus-4-6",
		"opus-4.6":             "claude-opus-4-6",
		"opus-4-6-thinking":    "claude-opus-4-6-thinking",
		"claude-sonnet-4-6":    "claude-sonnet-4-6",
		"claude-opus-4-6":      "claude-opus-4-6",
		"claude-haiku-4-5":     "claude-haiku-4-5",
		"gemini-3-pro-preview": "gemini-3-pro-preview",
	}

	for input, expected := range tests {
		if got := CanonicalClaudeModelAlias(input); got != expected {
			t.Fatalf("CanonicalClaudeModelAlias(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestClaudeModelAliasCandidatesKeepCanonicalFirstAndAliasesAvailable(t *testing.T) {
	candidates := ClaudeModelAliasCandidates("claude-opus-4-6")
	if len(candidates) < 3 {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
	if candidates[0] != "claude-opus-4-6" {
		t.Fatalf("expected canonical model first, got %+v", candidates)
	}

	hasShortAlias := false
	for _, candidate := range candidates {
		if candidate == "opus-4-6" {
			hasShortAlias = true
			break
		}
	}
	if !hasShortAlias {
		t.Fatalf("expected short alias to remain accepted for compatibility, got %+v", candidates)
	}
}
