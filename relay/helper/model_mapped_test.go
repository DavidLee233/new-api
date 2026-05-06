package helper

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func TestModelMappedHelper_AntigravityDefaultMapping(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-opus-4-6",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAntigravity,
			UpstreamModelName: "claude-opus-4-6",
		},
	}
	request := &dto.GeneralOpenAIRequest{Model: "claude-opus-4-6"}

	if err := ModelMappedHelper(c, info, request); err != nil {
		t.Fatalf("ModelMappedHelper returned error: %v", err)
	}
	if !info.IsModelMapped {
		t.Fatal("expected antigravity default mapping to mark model as mapped")
	}
	if info.UpstreamModelName != "claude-opus-4-6-thinking" {
		t.Fatalf("unexpected upstream model: %s", info.UpstreamModelName)
	}
	if request.Model != "claude-opus-4-6-thinking" {
		t.Fatalf("unexpected request model: %s", request.Model)
	}
}

func TestModelMappedHelper_AntigravityCustomMappingOverridesDefault(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(string(constant.ContextKeyChannelModelMapping), `{"claude-opus-4-6":"custom-opus"}`)
	c.Set("model_mapping", `{"claude-opus-4-6":"custom-opus"}`)

	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-opus-4-6",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAntigravity,
			UpstreamModelName: "claude-opus-4-6",
		},
	}
	request := &dto.GeneralOpenAIRequest{Model: "claude-opus-4-6"}

	if err := ModelMappedHelper(c, info, request); err != nil {
		t.Fatalf("ModelMappedHelper returned error: %v", err)
	}
	if info.UpstreamModelName != "custom-opus" {
		t.Fatalf("unexpected upstream model: %s", info.UpstreamModelName)
	}
	if request.Model != "custom-opus" {
		t.Fatalf("unexpected request model: %s", request.Model)
	}
}

func TestModelMappedHelper_AntigravityShortClaudeAliases(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		upstream string
	}{
		{
			name:     "sonnet short alias",
			model:    "sonnet-4-6",
			upstream: "claude-sonnet-4-6",
		},
		{
			name:     "sonnet official id",
			model:    "claude-sonnet-4-6",
			upstream: "claude-sonnet-4-6",
		},
		{
			name:     "opus short alias",
			model:    "opus-4-6",
			upstream: "claude-opus-4-6-thinking",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)

			info := &relaycommon.RelayInfo{
				OriginModelName: tt.model,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:       constant.ChannelTypeAntigravity,
					UpstreamModelName: tt.model,
				},
			}
			request := &dto.GeneralOpenAIRequest{Model: tt.model}

			if err := ModelMappedHelper(c, info, request); err != nil {
				t.Fatalf("ModelMappedHelper returned error: %v", err)
			}
			if info.UpstreamModelName != tt.upstream {
				t.Fatalf("unexpected upstream model: %s", info.UpstreamModelName)
			}
			if request.Model != tt.upstream {
				t.Fatalf("unexpected request model: %s", request.Model)
			}
		})
	}
}
