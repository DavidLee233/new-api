package constant

import "strings"

var defaultAntigravityModelMapping = map[string]string{
	// Claude aliases
	"opus-4-6-thinking":          "claude-opus-4-6-thinking",
	"opus-4.6-thinking":          "claude-opus-4-6-thinking",
	"opus-4-6":                   "claude-opus-4-6-thinking",
	"opus-4.6":                   "claude-opus-4-6-thinking",
	"opus-4-6-max":               "claude-opus-4-6-thinking",
	"opus-4-6-high":              "claude-opus-4-6-thinking",
	"opus-4-6-medium":            "claude-opus-4-6-thinking",
	"opus-4-6-low":               "claude-opus-4-6-thinking",
	"sonnet-4-6-thinking":        "claude-sonnet-4-6-thinking",
	"sonnet-4.6-thinking":        "claude-sonnet-4-6-thinking",
	"sonnet-4-6":                 "claude-sonnet-4-6",
	"sonnet-4.6":                 "claude-sonnet-4-6",
	"claude-opus-4-6-thinking":   "claude-opus-4-6-thinking",
	"claude-opus-4-6":            "claude-opus-4-6-thinking",
	"claude-opus-4.6":            "claude-opus-4-6-thinking",
	"claude-opus-4-6-max":        "claude-opus-4-6-thinking",
	"claude-opus-4-6-high":       "claude-opus-4-6-thinking",
	"claude-opus-4-6-medium":     "claude-opus-4-6-thinking",
	"claude-opus-4-6-low":        "claude-opus-4-6-thinking",
	"claude-opus-4-5-thinking":   "claude-opus-4-6-thinking",
	"claude-opus-4-5-20251101":   "claude-opus-4-6-thinking",
	"claude-opus-4.5":            "claude-opus-4-6-thinking",
	"claude-sonnet-4-6":          "claude-sonnet-4-6",
	"claude-sonnet-4-6-thinking": "claude-sonnet-4-6-thinking",
	"claude-sonnet-4.6":          "claude-sonnet-4-6",
	"claude-sonnet-4-5":          "claude-sonnet-4-5",
	"claude-sonnet-4.5":          "claude-sonnet-4-5",
	"claude-sonnet-4-5-thinking": "claude-sonnet-4-5-thinking",
	"claude-sonnet-4-5-20250929": "claude-sonnet-4-5",
	"claude-haiku-4-5":           "claude-sonnet-4-6",
	"claude-haiku-4.5":           "claude-sonnet-4-6",
	"claude-haiku-4-5-20251001":  "claude-sonnet-4-6",

	// Gemini aliases
	"gemini-2.5-flash":               "gemini-2.5-flash",
	"gemini-2.5-flash-image":         "gemini-2.5-flash-image",
	"gemini-2.5-flash-image-preview": "gemini-2.5-flash-image",
	"gemini-2.5-flash-lite":          "gemini-2.5-flash-lite",
	"gemini-2.5-flash-thinking":      "gemini-2.5-flash-thinking",
	"gemini-2.5-pro":                 "gemini-2.5-pro",
	"gemini-3-flash":                 "gemini-3-flash",
	"gemini-3-pro-high":              "gemini-3-pro-high",
	"gemini-3-pro-low":               "gemini-3-pro-low",
	"gemini-3-flash-preview":         "gemini-3-flash",
	"gemini-3-pro-preview":           "gemini-3-pro-high",
	"gemini-3.1-pro-high":            "gemini-3.1-pro-high",
	"gemini-3.1-pro-low":             "gemini-3.1-pro-low",
	"gemini-3.1-pro-preview":         "gemini-3.1-pro-high",
	"gemini-3.1-flash-image":         "gemini-3.1-flash-image",
	"gemini-3.1-flash-image-preview": "gemini-3.1-flash-image",
	"gemini-3-pro-image":             "gemini-3.1-flash-image",
	"gemini-3-pro-image-preview":     "gemini-3.1-flash-image",

	// Other official models
	"gpt-oss-120b-medium":    "gpt-oss-120b-medium",
	"tab_flash_lite_preview": "tab_flash_lite_preview",
}

func DefaultAntigravityModelMapping() map[string]string {
	result := make(map[string]string, len(defaultAntigravityModelMapping))
	for source, target := range defaultAntigravityModelMapping {
		result[source] = target
	}
	return result
}

func MergeAntigravityModelMapping(custom map[string]string) map[string]string {
	merged := DefaultAntigravityModelMapping()
	for source, target := range custom {
		normalizedSource := strings.TrimSpace(source)
		normalizedTarget := strings.TrimSpace(target)
		if normalizedSource == "" || normalizedTarget == "" {
			continue
		}
		merged[normalizedSource] = normalizedTarget
	}
	return merged
}

func CanonicalClaudeModelAlias(model string) string {
	model = strings.TrimSpace(model)
	switch model {
	case "sonnet-4-6", "sonnet-4.6":
		return "claude-sonnet-4-6"
	case "sonnet-4-6-thinking", "sonnet-4.6-thinking":
		return "claude-sonnet-4-6-thinking"
	case "opus-4-6", "opus-4.6":
		return "claude-opus-4-6"
	case "opus-4-6-thinking", "opus-4.6-thinking":
		return "claude-opus-4-6-thinking"
	case "opus-4-6-max":
		return "claude-opus-4-6-max"
	case "opus-4-6-high":
		return "claude-opus-4-6-high"
	case "opus-4-6-medium":
		return "claude-opus-4-6-medium"
	case "opus-4-6-low":
		return "claude-opus-4-6-low"
	default:
		return model
	}
}

func ClaudeModelAliasCandidates(model string) []string {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}

	candidates := []string{model}
	appendCandidate := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		for _, existing := range candidates {
			if existing == candidate {
				return
			}
		}
		candidates = append(candidates, candidate)
	}

	canonical := CanonicalClaudeModelAlias(model)
	appendCandidate(canonical)
	switch canonical {
	case "claude-sonnet-4-6":
		appendCandidate("sonnet-4-6")
		appendCandidate("sonnet-4.6")
	case "claude-sonnet-4-6-thinking":
		appendCandidate("sonnet-4-6-thinking")
		appendCandidate("sonnet-4.6-thinking")
	case "claude-opus-4-6":
		appendCandidate("opus-4-6")
		appendCandidate("opus-4.6")
	case "claude-opus-4-6-thinking":
		appendCandidate("opus-4-6-thinking")
		appendCandidate("opus-4.6-thinking")
	case "claude-opus-4-6-max":
		appendCandidate("opus-4-6-max")
	case "claude-opus-4-6-high":
		appendCandidate("opus-4-6-high")
	case "claude-opus-4-6-medium":
		appendCandidate("opus-4-6-medium")
	case "claude-opus-4-6-low":
		appendCandidate("opus-4-6-low")
	}
	return candidates
}
