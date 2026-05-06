package ratio_setting_test

import (
	"testing"

	relaycodex "github.com/QuantumNous/new-api/relay/channel/codex"
	relayopenai "github.com/QuantumNous/new-api/relay/channel/openai"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func TestOpenAICommonModelListHasDefaultModelRatio(t *testing.T) {
	ratio_setting.InitRatioSettings()

	models := []string{
		"gpt-5",
		"gpt-5-codex",
		"gpt-5-codex-mini",
		"gpt-5-pro",
		"gpt-5-search-api",
		"gpt-5.1",
		"gpt-5.1-chat-latest",
		"gpt-5.1-codex",
		"gpt-5.1-codex-max",
		"gpt-5.1-codex-mini",
		"gpt-5.2",
		"gpt-5.2-chat-latest",
		"gpt-5.2-pro",
		"gpt-5.2-codex",
		"gpt-5.3-chat-latest",
		"gpt-5.3-codex",
		"gpt-5.3-codex-spark",
		"gpt-5.4",
		"gpt-5.4-pro",
	}

	for _, modelName := range models {
		if _, ok, matchName := ratio_setting.GetModelRatio(modelName); !ok {
			t.Fatalf("GetModelRatio(%q) not configured, matchName=%q", modelName, matchName)
		}
	}
}

func TestCodexModelListHasDefaultModelRatio(t *testing.T) {
	ratio_setting.InitRatioSettings()

	for _, modelName := range relaycodex.ModelList {
		if _, ok, matchName := ratio_setting.GetModelRatio(modelName); !ok {
			t.Fatalf("GetModelRatio(%q) not configured, matchName=%q", modelName, matchName)
		}
	}
}

func TestOpenAIModelListHasDefaultModelRatioForKeyNewFamilies(t *testing.T) {
	ratio_setting.InitRatioSettings()

	required := map[string]bool{
		"gpt-5-codex":      false,
		"gpt-5.1-codex":    false,
		"gpt-5.2-codex":    false,
		"gpt-5.3-codex":    false,
		"gpt-5.4":          false,
		"gpt-5.4-pro":      false,
		"gpt-5-search-api": false,
	}

	for _, modelName := range relayopenai.ModelList {
		if _, exists := required[modelName]; exists {
			if _, ok, matchName := ratio_setting.GetModelRatio(modelName); !ok {
				t.Fatalf("GetModelRatio(%q) not configured, matchName=%q", modelName, matchName)
			}
			required[modelName] = true
		}
	}

	for modelName, seen := range required {
		if !seen {
			t.Fatalf("required OpenAI model %q not found in OpenAI model list", modelName)
		}
	}
}

func TestUpdateModelRatioPreservesNewOpenAIDefaults(t *testing.T) {
	ratio_setting.InitRatioSettings()

	if err := ratio_setting.UpdateModelRatioByJSONString(`{"gpt-4o":9.9}`); err != nil {
		t.Fatalf("UpdateModelRatioByJSONString() error = %v", err)
	}

	if ratio, ok, _ := ratio_setting.GetModelRatio("gpt-5.3-codex"); !ok || ratio != 0.625 {
		t.Fatalf("gpt-5.3-codex ratio = %v, %v; want 0.625, true", ratio, ok)
	}
	if ratio, ok, _ := ratio_setting.GetModelRatio("gpt-5.3-codex-openai-compact"); !ok || ratio != 0.625 {
		t.Fatalf("gpt-5.3-codex-openai-compact ratio = %v, %v; want 0.625, true", ratio, ok)
	}
	if ratio, ok, _ := ratio_setting.GetModelRatio("gpt-4o"); !ok || ratio != 9.9 {
		t.Fatalf("gpt-4o ratio = %v, %v; want 9.9, true", ratio, ok)
	}
}
