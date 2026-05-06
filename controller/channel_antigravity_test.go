package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func TestSelectChannelTestModelPrefersStableAntigravityTextModel(t *testing.T) {
	channel := &model.Channel{
		Type:   constant.ChannelTypeAntigravity,
		Models: "gemini-3-pro-image,claude-sonnet-4-6,gemini-2.5-flash",
	}

	if got := selectChannelTestModel(channel); got != "claude-sonnet-4-6" {
		t.Fatalf("selectChannelTestModel() = %q, want %q", got, "claude-sonnet-4-6")
	}
}

func TestSelectChannelTestModelKeepsExplicitTestModel(t *testing.T) {
	testModel := "gemini-3-pro-image"
	channel := &model.Channel{
		Type:      constant.ChannelTypeAntigravity,
		Models:    "claude-sonnet-4-6,gemini-2.5-flash",
		TestModel: &testModel,
	}

	if got := selectChannelTestModel(channel); got != testModel {
		t.Fatalf("selectChannelTestModel() = %q, want %q", got, testModel)
	}
}
