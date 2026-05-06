package antigravity

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

func buildAntigravityImageRequest(request dto.ImageRequest) (*dto.GeminiChatRequest, error) {
	if strings.TrimSpace(request.Prompt) == "" {
		return nil, errors.New("antigravity channel: prompt is required")
	}

	imageConfig := map[string]any{
		"aspectRatio": antigravityAspectRatioFromSize(request.Size),
		"imageSize":   antigravityImageSizeFromRequest(request),
	}

	imageConfigBytes, err := common.Marshal(imageConfig)
	if err != nil {
		return nil, err
	}

	geminiRequest := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role: "user",
				Parts: []dto.GeminiPart{
					{Text: request.Prompt},
				},
			},
		},
		GenerationConfig: dto.GeminiChatGenerationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
			ImageConfig:        imageConfigBytes,
		},
	}

	if request.N != nil && lo.FromPtr(request.N) > 1 {
		candidateCount := 1
		geminiRequest.GenerationConfig.CandidateCount = &candidateCount
	}

	return geminiRequest, nil
}

func antigravityAspectRatioFromSize(size string) string {
	size = strings.TrimSpace(size)
	if size == "" {
		return "1:1"
	}
	if strings.Contains(size, ":") {
		return size
	}
	switch size {
	case "256x256", "512x512", "1024x1024":
		return "1:1"
	case "1536x1024":
		return "3:2"
	case "1024x1536":
		return "2:3"
	case "1024x1792":
		return "9:16"
	case "1792x1024":
		return "16:9"
	default:
		return "1:1"
	}
}

func antigravityImageSizeFromRequest(request dto.ImageRequest) string {
	switch strings.ToLower(strings.TrimSpace(request.Quality)) {
	case "", "auto", "standard", "medium", "2k":
		return "2K"
	case "hd", "high", "4k":
		return "4K"
	case "low", "standard-1k", "1k":
		return "1K"
	default:
		return "2K"
	}
}

func antigravityImageHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	var geminiResponse dto.GeminiChatResponse
	if err = common.Unmarshal(responseBody, &geminiResponse); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	imageResponse := dto.ImageResponse{
		Created: common.GetTimestamp(),
		Data:    make([]dto.ImageData, 0, len(geminiResponse.Candidates)),
	}

	for _, candidate := range geminiResponse.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData == nil || !strings.HasPrefix(strings.ToLower(part.InlineData.MimeType), "image/") {
				continue
			}
			imageResponse.Data = append(imageResponse.Data, dto.ImageData{
				B64Json: part.InlineData.Data,
			})
		}
	}

	if len(imageResponse.Data) == 0 {
		return nil, types.NewOpenAIError(errors.New("no images generated"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	resp.Header.Set("Content-Type", "application/json")
	jsonResponse, err := common.Marshal(imageResponse)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	service.IOCopyBytesGracefully(c, resp, jsonResponse)

	usage := buildAntigravityUsage(geminiResponse.UsageMetadata, info.GetEstimatePromptTokens())
	if usage.TotalTokens == 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
		usage.CompletionTokens = len(imageResponse.Data) * 1400
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return &usage, nil
}

func buildAntigravityUsage(metadata dto.GeminiUsageMetadata, fallbackPromptTokens int) dto.Usage {
	promptTokens := metadata.PromptTokenCount + metadata.ToolUsePromptTokenCount
	if promptTokens <= 0 && fallbackPromptTokens > 0 {
		promptTokens = fallbackPromptTokens
	}

	usage := dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: metadata.CandidatesTokenCount + metadata.ThoughtsTokenCount,
		TotalTokens:      metadata.TotalTokenCount,
	}
	usage.CompletionTokenDetails.ReasoningTokens = metadata.ThoughtsTokenCount
	usage.PromptTokensDetails.CachedTokens = metadata.CachedContentTokenCount

	if usage.TotalTokens > 0 && usage.CompletionTokens <= 0 {
		usage.CompletionTokens = usage.TotalTokens - usage.PromptTokens
	}
	return usage
}

func isImageGenerationModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	model = strings.TrimPrefix(model, "models/")

	return model == "gemini-3.1-flash-image" ||
		model == "gemini-3.1-flash-image-preview" ||
		strings.HasPrefix(model, "gemini-3.1-flash-image-") ||
		model == "gemini-3-pro-image" ||
		model == "gemini-3-pro-image-preview" ||
		strings.HasPrefix(model, "gemini-3-pro-image-") ||
		model == "gemini-2.5-flash-image" ||
		model == "gemini-2.5-flash-image-preview" ||
		strings.HasPrefix(model, "gemini-2.5-flash-image-")
}
