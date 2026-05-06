package antigravity

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	geminiAdaptor gemini.Adaptor
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.geminiAdaptor.Init(info)
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return buildRequestURLWithBase(info.ChannelBaseUrl, info)
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	oauthKey, err := parseOAuthKey(info.ApiKey)
	if err != nil {
		return err
	}
	if strings.TrimSpace(oauthKey.AccessToken) == "" {
		return errors.New("antigravity channel: access_token is required")
	}

	req.Set("Authorization", "Bearer "+strings.TrimSpace(oauthKey.AccessToken))
	req.Set("User-Agent", getUserAgent())
	req.Set("Content-Type", "application/json")
	if info.IsStream {
		req.Set("Accept", "text/event-stream")
	} else if req.Get("Accept") == "" {
		req.Set("Accept", "application/json")
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	converted, err := a.geminiAdaptor.ConvertOpenAIRequest(c, info, request)
	if err != nil {
		return nil, err
	}
	return a.wrapGeminiPayload(converted, info)
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	converted, err := a.geminiAdaptor.ConvertClaudeRequest(c, info, request)
	if err != nil {
		return nil, err
	}
	return a.wrapGeminiPayload(converted, info)
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	converted, err := a.geminiAdaptor.ConvertGeminiRequest(c, info, request)
	if err != nil {
		return nil, err
	}
	return a.wrapGeminiPayload(converted, info)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("antigravity channel: /v1/rerank endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("antigravity channel: /v1/embeddings endpoint not supported")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("antigravity channel: audio endpoint not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if !isImageGenerationModel(info.UpstreamModelName) {
		return nil, fmt.Errorf("antigravity channel: image endpoint only supports Antigravity image models, got %s", info.UpstreamModelName)
	}

	converted, err := buildAntigravityImageRequest(request)
	if err != nil {
		return nil, err
	}
	return a.wrapGeminiPayload(converted, info)
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("antigravity channel: /v1/responses endpoint not supported")
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	transformedResp, unwrapErr := unwrapResponse(resp, info.IsStream)
	if unwrapErr != nil {
		return nil, types.NewError(unwrapErr, types.ErrorCodeBadResponseBody)
	}

	if info.RelayMode == relayconstant.RelayModeImagesGenerations {
		return antigravityImageHandler(c, info, transformedResp)
	}

	if info.RelayMode == relayconstant.RelayModeGemini {
		if info.IsStream {
			return gemini.GeminiTextGenerationStreamHandler(c, info, transformedResp)
		}
		return gemini.GeminiTextGenerationHandler(c, info, transformedResp)
	}

	if info.IsStream {
		return gemini.GeminiChatStreamHandler(c, info, transformedResp)
	}
	return gemini.GeminiChatHandler(c, info, transformedResp)
}

func (a *Adaptor) GetModelList() []string {
	return nil
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) wrapGeminiPayload(payload any, info *relaycommon.RelayInfo) (any, error) {
	oauthKey, err := parseOAuthKey(info.ApiKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(oauthKey.ProjectID) == "" {
		return nil, errors.New("antigravity channel: project_id is required")
	}

	payloadBytes, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}

	payloadBytes, err = injectIdentityPatch(payloadBytes)
	if err != nil {
		return nil, err
	}
	payloadBytes, err = cleanGeminiRequest(payloadBytes)
	if err != nil {
		return nil, err
	}

	return wrapV1InternalRequest(strings.TrimSpace(oauthKey.ProjectID), info.UpstreamModelName, payloadBytes)
}

func unwrapResponse(resp *http.Response, isStream bool) (*http.Response, error) {
	if resp == nil || resp.Body == nil {
		return resp, nil
	}

	if isStream {
		resp.Body = newStreamUnwrapper(resp.Body)
		resp.ContentLength = -1
		resp.Header.Del("Content-Length")
		return resp, nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	unwrapped, err := unwrapV1InternalResponse(bodyBytes)
	if err != nil {
		return nil, err
	}

	resp.Body = io.NopCloser(bytes.NewReader(unwrapped))
	resp.ContentLength = int64(len(unwrapped))
	resp.Header.Del("Content-Length")
	return resp, nil
}
