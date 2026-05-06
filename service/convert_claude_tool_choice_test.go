package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestClaudeToOpenAIRequestConvertsToolChoiceAnyToRequired(t *testing.T) {
	t.Parallel()

	disabledParallel := true
	req := dto.ClaudeRequest{
		Model: "claude-sonnet-4-6",
		ToolChoice: map[string]any{
			"type":                      "any",
			"disable_parallel_tool_use": disabledParallel,
		},
	}

	openAIReq, err := ClaudeToOpenAIRequest(req, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	})

	require.NoError(t, err)
	require.Equal(t, "required", openAIReq.ToolChoice)
	require.NotNil(t, openAIReq.ParallelTooCalls)
	require.False(t, *openAIReq.ParallelTooCalls)
}

func TestClaudeToOpenAIRequestConvertsSpecificToolChoice(t *testing.T) {
	t.Parallel()

	req := dto.ClaudeRequest{
		Model: "claude-sonnet-4-6",
		ToolChoice: map[string]any{
			"type": "tool",
			"name": "Read",
		},
	}

	openAIReq, err := ClaudeToOpenAIRequest(req, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	})

	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"type": "function",
		"function": map[string]any{
			"name": "Read",
		},
	}, openAIReq.ToolChoice)
}

func TestNormalizeClaudeToolUseIDFallbackAndSanitize(t *testing.T) {
	t.Parallel()

	require.Equal(t, "call_bad_id", normalizeClaudeToolUseID("call bad/id"))
	require.NotEmpty(t, normalizeClaudeToolUseID(""))
}
