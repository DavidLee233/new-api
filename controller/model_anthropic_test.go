package controller

import "testing"

func TestBuildAnthropicModelUsesCanonicalIDAsDisplayName(t *testing.T) {
	model := buildAnthropicModel("sonnet-4-6", 1626777600)

	if model.ID != "claude-sonnet-4-6" {
		t.Fatalf("unexpected model id: %s", model.ID)
	}
	if model.DisplayName != "claude-sonnet-4-6" {
		t.Fatalf("unexpected display name: %s", model.DisplayName)
	}
}

func TestBuildAnthropicModelCanonicalizesOpusAlias(t *testing.T) {
	model := buildAnthropicModel("opus-4-6", 1626777600)

	if model.ID != "claude-opus-4-6" {
		t.Fatalf("unexpected model id: %s", model.ID)
	}
	if model.DisplayName != "claude-opus-4-6" {
		t.Fatalf("unexpected display name: %s", model.DisplayName)
	}
}
