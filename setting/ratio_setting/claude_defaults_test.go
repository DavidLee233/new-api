package ratio_setting_test

import (
	"testing"

	relayclaude "github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func TestClaudeModelListHasDefaultModelRatio(t *testing.T) {
	ratio_setting.InitRatioSettings()

	for _, modelName := range relayclaude.ModelList {
		if _, ok, matchName := ratio_setting.GetModelRatio(modelName); !ok {
			t.Fatalf("GetModelRatio(%q) not configured, matchName=%q", modelName, matchName)
		}
	}
}

func TestClaudeModelListHasDefaultCacheRatios(t *testing.T) {
	ratio_setting.InitRatioSettings()

	for _, modelName := range relayclaude.ModelList {
		if _, ok := ratio_setting.GetCacheRatio(modelName); !ok {
			t.Fatalf("GetCacheRatio(%q) not configured", modelName)
		}
		if _, ok := ratio_setting.GetCreateCacheRatio(modelName); !ok {
			t.Fatalf("GetCreateCacheRatio(%q) not configured", modelName)
		}
	}
}

func TestUpdateModelRatioPreservesNewClaudeDefaults(t *testing.T) {
	ratio_setting.InitRatioSettings()

	if err := ratio_setting.UpdateModelRatioByJSONString(`{"gpt-4o":9.9}`); err != nil {
		t.Fatalf("UpdateModelRatioByJSONString() error = %v", err)
	}

	if ratio, ok, _ := ratio_setting.GetModelRatio("claude-sonnet-4-6"); !ok || ratio != 1.5 {
		t.Fatalf("claude-sonnet-4-6 ratio = %v, %v; want 1.5, true", ratio, ok)
	}
	if ratio, ok, _ := ratio_setting.GetModelRatio("gpt-4o"); !ok || ratio != 9.9 {
		t.Fatalf("gpt-4o ratio = %v, %v; want 9.9, true", ratio, ok)
	}
}

func TestClaudeShortAliasesNormalizeForMatching(t *testing.T) {
	tests := map[string]string{
		"sonnet-4-6":      "claude-sonnet-4-6",
		"sonnet-4.6":      "claude-sonnet-4-6",
		"opus-4-6":        "claude-opus-4-6",
		"opus-4.6":        "claude-opus-4-6",
		"opus-4-6-high":   "claude-opus-4-6-high",
		"opus-4-6-medium": "claude-opus-4-6-medium",
	}

	for alias, want := range tests {
		if got := ratio_setting.FormatMatchingModelName(alias); got != want {
			t.Fatalf("FormatMatchingModelName(%q) = %q, want %q", alias, got, want)
		}
	}
}

func TestClaudeAliasCandidatesIncludeCanonicalAndShortNames(t *testing.T) {
	candidates := ratio_setting.FormatMatchingModelNameCandidates("claude-opus-4-6")
	required := map[string]bool{
		"claude-opus-4-6": false,
		"opus-4-6":        false,
		"opus-4.6":        false,
	}

	for _, candidate := range candidates {
		if _, ok := required[candidate]; ok {
			required[candidate] = true
		}
	}
	for modelName, found := range required {
		if !found {
			t.Fatalf("candidate %q not found in %v", modelName, candidates)
		}
	}
}
