package antigravity

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type antigravityErrorPayload struct {
	Error antigravityErrorObject `json:"error"`
}

type antigravityErrorObject struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Status  string           `json:"status"`
	Details []map[string]any `json:"details"`
}

type antigravityErrorInfo struct {
	UpstreamCode             int
	UpstreamStatus           string
	Message                  string
	ModelName                string
	RetryDelay               time.Duration
	IsRateLimitExceeded      bool
	IsModelCapacityExhausted bool
	IsSafetyBlocked          bool
}

func BuildRelayError(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, showBodyWhenFail bool) *types.NewAPIError {
	if resp == nil {
		return types.NewOpenAIError(fmt.Errorf("bad response status code"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	parsed := parseAntigravityErrorInfo(responseBody)
	if parsed == nil {
		return service.RelayErrorHandler(getRequestContext(c), rebuildResponse(resp, responseBody), showBodyWhenFail)
	}

	if parsed.IsSafetyBlocked && c != nil {
		common.SetContextKey(c, constant.ContextKeyAdminRejectReason, fmt.Sprintf("antigravity_block_reason=%s", firstNonEmpty(parsed.UpstreamStatus, "SAFETY")))
	}

	openAIError, errorCode := mapAntigravityError(parsed, resp.StatusCode)
	openAIError.Code = string(errorCode)
	newAPIError := types.WithOpenAIError(openAIError, resp.StatusCode)
	if showBodyWhenFail {
		newAPIError.Err = fmt.Errorf("bad response status code %d, message: %s, body: %s", resp.StatusCode, openAIError.Message, string(responseBody))
	}
	return newAPIError
}

func parseAntigravityErrorInfo(body []byte) *antigravityErrorInfo {
	var payload antigravityErrorPayload
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil
	}

	if payload.Error.Status == "" && strings.TrimSpace(payload.Error.Message) == "" {
		return nil
	}

	info := &antigravityErrorInfo{
		UpstreamCode:   payload.Error.Code,
		UpstreamStatus: strings.TrimSpace(payload.Error.Status),
		Message:        strings.TrimSpace(payload.Error.Message),
	}

	for _, detail := range payload.Error.Details {
		atType := strings.TrimSpace(common.Interface2String(detail["@type"]))
		switch atType {
		case googleRPCTypeErrorInfo:
			if metadata, ok := detail["metadata"].(map[string]any); ok {
				if model := strings.TrimSpace(common.Interface2String(metadata["model"])); model != "" {
					info.ModelName = model
				}
			}
			reason := strings.TrimSpace(common.Interface2String(detail["reason"]))
			switch reason {
			case googleRPCReasonRateLimitExceeded:
				info.IsRateLimitExceeded = true
			case googleRPCReasonModelCapacityExhausted:
				info.IsModelCapacityExhausted = true
			case "SAFETY", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII", "RECITATION":
				info.IsSafetyBlocked = true
			}
		case googleRPCTypeRetryInfo:
			delayText := strings.TrimSpace(common.Interface2String(detail["retryDelay"]))
			if delayText == "" {
				continue
			}
			if retryDelay, err := time.ParseDuration(delayText); err == nil {
				info.RetryDelay = retryDelay
			}
		}
	}

	if info.RetryDelay <= 0 && (info.IsRateLimitExceeded || info.IsModelCapacityExhausted) {
		info.RetryDelay = antigravityDefaultRateLimitDuration
	}
	if !info.IsSafetyBlocked {
		info.IsSafetyBlocked = looksLikeSafetyBlock(info.Message)
	}
	return info
}

func mapAntigravityError(info *antigravityErrorInfo, statusCode int) (types.OpenAIError, types.ErrorCode) {
	message := strings.TrimSpace(info.Message)
	if message == "" {
		message = defaultAntigravityErrorMessage(info, statusCode)
	}

	errorType := "api_error"
	errorCode := types.ErrorCodeBadResponseStatusCode

	switch {
	case info.IsSafetyBlocked:
		errorType = "invalid_request_error"
		errorCode = types.ErrorCodePromptBlocked
	case strings.EqualFold(info.UpstreamStatus, "INVALID_ARGUMENT"),
		strings.EqualFold(info.UpstreamStatus, "FAILED_PRECONDITION"),
		strings.EqualFold(info.UpstreamStatus, "OUT_OF_RANGE"):
		errorType = "invalid_request_error"
		errorCode = types.ErrorCodeInvalidRequest
	case strings.EqualFold(info.UpstreamStatus, "NOT_FOUND"):
		errorType = "invalid_request_error"
		if strings.Contains(strings.ToLower(message), "model") || info.ModelName != "" {
			errorCode = types.ErrorCodeModelNotFound
		} else {
			errorCode = types.ErrorCodeInvalidRequest
		}
	case strings.EqualFold(info.UpstreamStatus, "UNAUTHENTICATED"):
		errorType = "authentication_error"
		errorCode = types.ErrorCodeChannelInvalidKey
		message = "Antigravity upstream authentication failed"
	case strings.EqualFold(info.UpstreamStatus, "PERMISSION_DENIED"):
		errorType = "permission_error"
		errorCode = types.ErrorCodeAccessDenied
		message = firstNonEmpty(message, "Antigravity upstream access forbidden")
	case info.IsRateLimitExceeded || strings.EqualFold(info.UpstreamStatus, googleRPCStatusResourceExhausted) || statusCode == http.StatusTooManyRequests:
		errorType = "rate_limit_error"
		errorCode = types.ErrorCodeBadResponseStatusCode
		message = buildRateLimitMessage(info, message)
	case info.IsModelCapacityExhausted:
		errorType = "overloaded_error"
		errorCode = types.ErrorCodeBadResponseStatusCode
		message = buildModelCapacityMessage(info, message)
	case strings.EqualFold(info.UpstreamStatus, googleRPCStatusUnavailable) || statusCode == http.StatusServiceUnavailable:
		errorType = "overloaded_error"
		errorCode = types.ErrorCodeBadResponseStatusCode
		message = firstNonEmpty(message, "Antigravity upstream service unavailable")
	}

	return types.OpenAIError{
		Message:  message,
		Type:     errorType,
		Code:     string(errorCode),
		Metadata: buildAntigravityErrorMetadata(info),
	}, errorCode
}

func defaultAntigravityErrorMessage(info *antigravityErrorInfo, statusCode int) string {
	switch {
	case info.IsSafetyBlocked:
		return "Request blocked by Antigravity safety policy"
	case strings.EqualFold(info.UpstreamStatus, "INVALID_ARGUMENT"),
		strings.EqualFold(info.UpstreamStatus, "FAILED_PRECONDITION"),
		strings.EqualFold(info.UpstreamStatus, "OUT_OF_RANGE"):
		return "Invalid Antigravity request"
	case strings.EqualFold(info.UpstreamStatus, "NOT_FOUND"):
		if info.ModelName != "" {
			return fmt.Sprintf("Antigravity model not found: %s", info.ModelName)
		}
		return "Requested Antigravity resource was not found"
	case strings.EqualFold(info.UpstreamStatus, "UNAUTHENTICATED"):
		return "Antigravity upstream authentication failed"
	case strings.EqualFold(info.UpstreamStatus, "PERMISSION_DENIED"):
		return "Antigravity upstream access forbidden"
	case info.IsRateLimitExceeded || statusCode == http.StatusTooManyRequests:
		return "Antigravity upstream rate limit exceeded"
	case info.IsModelCapacityExhausted:
		return "Antigravity upstream model capacity exhausted"
	case strings.EqualFold(info.UpstreamStatus, googleRPCStatusUnavailable) || statusCode == http.StatusServiceUnavailable:
		return "Antigravity upstream service unavailable"
	default:
		return fmt.Sprintf("Antigravity upstream error (%d)", statusCode)
	}
}

func buildRateLimitMessage(info *antigravityErrorInfo, fallback string) string {
	if info == nil {
		return fallback
	}
	if info.ModelName != "" && info.RetryDelay > 0 {
		return fmt.Sprintf("Antigravity upstream rate limit exceeded for %s, retry after %s", info.ModelName, info.RetryDelay)
	}
	if info.ModelName != "" {
		return fmt.Sprintf("Antigravity upstream rate limit exceeded for %s", info.ModelName)
	}
	if info.RetryDelay > 0 {
		return fmt.Sprintf("Antigravity upstream rate limit exceeded, retry after %s", info.RetryDelay)
	}
	return firstNonEmpty(fallback, "Antigravity upstream rate limit exceeded")
}

func buildModelCapacityMessage(info *antigravityErrorInfo, fallback string) string {
	if info == nil {
		return fallback
	}
	if info.ModelName != "" && info.RetryDelay > 0 {
		return fmt.Sprintf("Antigravity model capacity exhausted for %s, retry after %s", info.ModelName, info.RetryDelay)
	}
	if info.ModelName != "" {
		return fmt.Sprintf("Antigravity model capacity exhausted for %s", info.ModelName)
	}
	return firstNonEmpty(fallback, "Antigravity upstream model capacity exhausted")
}

func buildAntigravityErrorMetadata(info *antigravityErrorInfo) []byte {
	if info == nil {
		return nil
	}

	metadata := map[string]any{}
	if info.UpstreamCode > 0 {
		metadata["upstream_code"] = info.UpstreamCode
	}
	if info.UpstreamStatus != "" {
		metadata["upstream_status"] = info.UpstreamStatus
	}
	if info.ModelName != "" {
		metadata["model"] = info.ModelName
	}
	if info.RetryDelay > 0 {
		metadata["retry_delay"] = info.RetryDelay.String()
	}
	if info.IsRateLimitExceeded {
		metadata["rate_limit_exceeded"] = true
	}
	if info.IsModelCapacityExhausted {
		metadata["model_capacity_exhausted"] = true
	}
	if info.IsSafetyBlocked {
		metadata["safety_blocked"] = true
	}
	if len(metadata) == 0 {
		return nil
	}

	raw, err := common.Marshal(metadata)
	if err != nil {
		return nil
	}
	return raw
}

func looksLikeSafetyBlock(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return false
	}
	keywords := []string{"safety", "blocked", "blocklist", "prohibited", "spii", "recitation"}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func getRequestContext(c *gin.Context) context.Context {
	if c != nil && c.Request != nil {
		return c.Request.Context()
	}
	return context.Background()
}
