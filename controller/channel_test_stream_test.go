package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func TestNormalizeChannelTestStream_CodexResponsesForcesStream(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeCodex}

	if !normalizeChannelTestStream(channel, "gpt-5.3-codex", "", false) {
		t.Fatalf("expected codex response test to force stream=true")
	}
	if !normalizeChannelTestStream(channel, "gpt-5.3-codex", string(constant.EndpointTypeOpenAIResponse), false) {
		t.Fatalf("expected explicit codex response endpoint to force stream=true")
	}
}

func TestNormalizeChannelTestStream_CodexCompactDoesNotForceStream(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeCodex}
	modelName := ratio_setting.WithCompactModelSuffix("gpt-5.3-codex")

	if normalizeChannelTestStream(channel, modelName, string(constant.EndpointTypeOpenAIResponseCompact), false) {
		t.Fatalf("expected codex compact response test to keep stream=false")
	}
}

func TestNormalizeChannelTestStream_NonCodexPreservesValue(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI}

	if normalizeChannelTestStream(channel, "gpt-5.3-codex", "", false) {
		t.Fatalf("expected non-codex channel test to keep stream=false")
	}
	if !normalizeChannelTestStream(channel, "gpt-4o", "", true) {
		t.Fatalf("expected explicit stream=true to be preserved")
	}
}
