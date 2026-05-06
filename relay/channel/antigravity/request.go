package antigravity

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type antigravitySmartRetryInfo struct {
	RetryDelay               time.Duration
	ModelName                string
	IsModelCapacityExhausted bool
}

var antigravityInvalidToolSchemaPattern = regexp.MustCompile(`tools\.(\d+)\.custom\.input_schema:\s*(?:JSON schema is invalid|Field required)`)

func (a *Adaptor) doRequestOnce(c *gin.Context, info *relaycommon.RelayInfo, body []byte, requestURL string, headerOverride map[string]string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}

	headers := req.Header
	if err = a.SetupRequestHeader(c, &headers, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}
	filteredHeaderOverride := filterAntigravityHeaderOverride(headerOverride)
	applyRequestHeaderOverride(req, filteredHeaderOverride)
	enforceAntigravityHeaders(req, info)
	logAntigravityHTTPDebug(c, req, filteredHeaderOverride)

	resp, err := client.Do(req)
	if err != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, err
	}

	if req.Body != nil {
		_ = req.Body.Close()
	}
	if c.Request != nil && c.Request.Body != nil {
		_ = c.Request.Body.Close()
	}
	logAntigravityResponseDebug(c, req, resp)
	return resp, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	body, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, fmt.Errorf("read request body failed: %w", err)
	}

	headerOverride, err := channel.ResolveHeaderOverride(info, c)
	if err != nil {
		return nil, err
	}

	client, err := service.GetHttpClientWithProxy(info.ChannelSetting.Proxy)
	if err != nil {
		return nil, err
	}

	baseURLs := buildBaseURLCandidates(info.ChannelBaseUrl)
	var lastErr error
	credentialRefreshAttempted := false
	for index, baseURL := range baseURLs {
		requestURL, urlErr := buildRequestURLWithBase(baseURL, info)
		if urlErr != nil {
			lastErr = urlErr
			continue
		}

		retryCount := 0
		downgradedToolSchemaIndexes := make(map[int]struct{})
		for {
			logAntigravityRequestDebug(c, info, requestURL, body)
			resp, reqErr := a.doRequestOnce(c, info, body, requestURL, headerOverride, client)
			if reqErr != nil {
				lastErr = reqErr
				if isContextDone(c.Request.Context(), reqErr) || index == len(baseURLs)-1 {
					return nil, reqErr
				}
				break
			}

			if shouldRefreshCredentialForUnauthorized(resp, credentialRefreshAttempted) {
				respBody, readErr := io.ReadAll(resp.Body)
				service.CloseResponseBodyGracefully(resp)
				if readErr != nil {
					return nil, fmt.Errorf("read antigravity unauthorized response failed: %w", readErr)
				}
				credentialRefreshAttempted = true
				if refreshErr := refreshAntigravityCredentialForRetry(c, info); refreshErr != nil {
					return rebuildResponse(resp, respBody), nil
				}
				resp, reqErr = a.doRequestOnce(c, info, body, requestURL, headerOverride, client)
				if reqErr != nil {
					lastErr = reqErr
					if isContextDone(c.Request.Context(), reqErr) || index == len(baseURLs)-1 {
						return nil, reqErr
					}
					break
				}
			}

			if resp.StatusCode == http.StatusBadRequest {
				respBody, readErr := io.ReadAll(resp.Body)
				service.CloseResponseBodyGracefully(resp)
				if readErr != nil {
					return nil, fmt.Errorf("read antigravity bad request response failed: %w", readErr)
				}
				if toolIndex, ok := extractAntigravityInvalidToolSchemaIndex(respBody); ok {
					if _, alreadyDowngraded := downgradedToolSchemaIndexes[toolIndex]; !alreadyDowngraded {
						downgradedBody, toolName, downgraded, downgradeErr := downgradeAntigravityToolSchema(body, toolIndex)
						if downgradeErr != nil {
							return nil, downgradeErr
						}
						if downgraded {
							downgradedToolSchemaIndexes[toolIndex] = struct{}{}
							body = downgradedBody
							logger.LogWarn(c, fmt.Sprintf("antigravity tool schema invalid; downgraded tool schema and retrying: index=%d name=%s", toolIndex, toolName))
							continue
						}
					}
				}
				return rebuildResponse(resp, respBody), nil
			}

			if shouldTryNextBaseURLForNotFound(resp, index, len(baseURLs)) {
				respBody, readErr := io.ReadAll(resp.Body)
				service.CloseResponseBodyGracefully(resp)
				if readErr != nil {
					return nil, fmt.Errorf("read antigravity not found response failed: %w", readErr)
				}
				lastErr = fmt.Errorf("antigravity upstream returned not found at %s: %s", baseURL, strings.TrimSpace(string(respBody)))
				break
			}

			if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusServiceUnavailable {
				return resp, nil
			}

			respBody, readErr := io.ReadAll(resp.Body)
			service.CloseResponseBodyGracefully(resp)
			if readErr != nil {
				return nil, fmt.Errorf("read antigravity retry response failed: %w", readErr)
			}

			retryInfo := parseAntigravitySmartRetryInfo(respBody)
			if retryInfo == nil {
				if resp.StatusCode == http.StatusServiceUnavailable && index < len(baseURLs)-1 {
					lastErr = fmt.Errorf("antigravity upstream unavailable at %s", baseURL)
					break
				}
				return rebuildResponse(resp, respBody), nil
			}

			waitDuration, shouldRetrySameURL, shouldTryNextBaseURL := classifySmartRetry(resp.StatusCode, retryInfo, retryCount)
			if shouldRetrySameURL {
				retryCount++
				if err = waitContext(c.Request.Context(), waitDuration); err != nil {
					return nil, err
				}
				continue
			}
			if shouldTryNextBaseURL && index < len(baseURLs)-1 {
				lastErr = fmt.Errorf("antigravity upstream throttled at %s for model %s", baseURL, retryInfo.ModelName)
				break
			}
			return rebuildResponse(resp, respBody), nil
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("antigravity request failed")
}

func logAntigravityRequestDebug(c *gin.Context, info *relaycommon.RelayInfo, requestURL string, body []byte) {
	if !common.DebugEnabled {
		return
	}
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		logger.LogDebug(c, "antigravity request debug: url=%s upstream_model=%s body_unmarshal_error=%v", requestURL, safeRelayModel(info), err)
		return
	}
	modelName := strings.TrimSpace(common.Interface2String(payload["model"]))
	requestType := strings.TrimSpace(common.Interface2String(payload["requestType"]))
	requestMap, _ := payload["request"].(map[string]any)
	contentsCount := 0
	if contents, ok := requestMap["contents"].([]any); ok {
		contentsCount = len(contents)
	}
	systemInstruction := "false"
	if _, ok := requestMap["systemInstruction"]; ok {
		systemInstruction = "true"
	}
	toolsCount := 0
	toolsSummary := ""
	if tools, ok := requestMap["tools"].([]any); ok {
		toolsCount = len(tools)
		toolsSummary = summarizeAntigravityTools(tools)
	}
	generationKeys := make([]string, 0)
	if generationConfig, ok := requestMap["generationConfig"].(map[string]any); ok {
		for key := range generationConfig {
			generationKeys = append(generationKeys, key)
		}
	}
	logger.LogDebug(c, "antigravity request debug: url=%s origin_model=%s upstream_model=%s wrapper_model=%s request_type=%s contents=%d system=%s tools=%d tool_summary=%s generation_keys=%s body_bytes=%d",
		requestURL,
		safeRelayOriginModel(info),
		safeRelayModel(info),
		modelName,
		requestType,
		contentsCount,
		systemInstruction,
		toolsCount,
		toolsSummary,
		strings.Join(generationKeys, ","),
		len(body),
	)
}

func summarizeAntigravityTools(tools []any) string {
	if len(tools) == 0 {
		return ""
	}
	items := make([]string, 0, len(tools))
	for i, tool := range tools {
		toolMap, ok := tool.(map[string]any)
		if !ok {
			items = append(items, fmt.Sprintf("%d:<%T>", i, tool))
			continue
		}
		keys := make([]string, 0, len(toolMap))
		for key := range toolMap {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if functions, ok := toolMap["functionDeclarations"].([]any); ok {
			items = append(items, fmt.Sprintf("%d:keys=%s,functionDeclarations=%d,%s", i, strings.Join(keys, ","), len(functions), summarizeFunctionDeclarations(functions)))
			continue
		}
		if custom, ok := toolMap["custom"].(map[string]any); ok {
			customKeys := make([]string, 0, len(custom))
			for key := range custom {
				customKeys = append(customKeys, key)
			}
			sort.Strings(customKeys)
			items = append(items, fmt.Sprintf("%d:keys=%s,custom_keys=%s", i, strings.Join(keys, ","), strings.Join(customKeys, ",")))
			continue
		}
		items = append(items, fmt.Sprintf("%d:keys=%s", i, strings.Join(keys, ",")))
	}
	return strings.Join(items, "|")
}

func summarizeFunctionDeclarations(functions []any) string {
	if len(functions) == 0 {
		return ""
	}
	items := make([]string, 0, len(functions))
	for i, function := range functions {
		fnMap, ok := function.(map[string]any)
		if !ok {
			items = append(items, fmt.Sprintf("fn%d=<%T>", i, function))
			continue
		}
		name := strings.TrimSpace(common.Interface2String(fnMap["name"]))
		parameters, hasParameters := fnMap["parameters"]
		items = append(items, fmt.Sprintf("fn%d:%s:parameters=%t:%s", i, name, hasParameters, summarizeSchema(parameters)))
	}
	return strings.Join(items, ",")
}

func summarizeSchema(schema any) string {
	schemaMap, ok := schema.(map[string]any)
	if !ok || schemaMap == nil {
		return ""
	}
	keys := make([]string, 0, len(schemaMap))
	for key := range schemaMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	propertiesCount := 0
	if properties, ok := schemaMap["properties"].(map[string]any); ok {
		propertiesCount = len(properties)
	}
	return fmt.Sprintf("type=%s,keys=%s,props=%d", common.Interface2String(schemaMap["type"]), strings.Join(keys, "+"), propertiesCount)
}

func extractAntigravityInvalidToolSchemaIndex(body []byte) (int, bool) {
	matches := antigravityInvalidToolSchemaPattern.FindSubmatch(body)
	if len(matches) < 2 {
		return 0, false
	}
	index, err := strconv.Atoi(string(matches[1]))
	if err != nil || index < 0 {
		return 0, false
	}
	return index, true
}

func downgradeAntigravityToolSchema(body []byte, toolIndex int) ([]byte, string, bool, error) {
	if toolIndex < 0 {
		return body, "", false, nil
	}

	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, "", false, fmt.Errorf("unmarshal antigravity request for tool schema downgrade failed: %w", err)
	}
	requestMap, ok := payload["request"].(map[string]any)
	if !ok {
		return body, "", false, nil
	}
	tools, ok := requestMap["tools"].([]any)
	if !ok {
		return body, "", false, nil
	}

	currentIndex := 0
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]any)
		if !ok {
			continue
		}
		functions, ok := toolMap["functionDeclarations"].([]any)
		if !ok {
			functions, _ = toolMap["function_declarations"].([]any)
		}
		for _, function := range functions {
			fnMap, ok := function.(map[string]any)
			if !ok {
				currentIndex++
				continue
			}
			if currentIndex == toolIndex {
				toolName := strings.TrimSpace(common.Interface2String(fnMap["name"]))
				fnMap["parameters"] = emptyAntigravityToolParameters()
				downgradedBody, err := common.Marshal(payload)
				if err != nil {
					return nil, "", false, fmt.Errorf("marshal antigravity request after tool schema downgrade failed: %w", err)
				}
				return downgradedBody, toolName, true, nil
			}
			currentIndex++
		}
	}
	return body, "", false, nil
}

func safeRelayModel(info *relaycommon.RelayInfo) string {
	if info == nil {
		return ""
	}
	return info.UpstreamModelName
}

func safeRelayOriginModel(info *relaycommon.RelayInfo) string {
	if info == nil {
		return ""
	}
	return info.OriginModelName
}

func logAntigravityHTTPDebug(c *gin.Context, req *http.Request, headerOverride map[string]string) {
	if !common.DebugEnabled || req == nil {
		return
	}
	logger.LogDebug(c, "antigravity http debug: method=%s url=%s host=%s headers=%s override_keys=%s",
		req.Method,
		req.URL.String(),
		req.Host,
		formatSafeHeaders(req.Header),
		formatHeaderOverrideKeys(headerOverride),
	)
}

func logAntigravityResponseDebug(c *gin.Context, req *http.Request, resp *http.Response) {
	if !common.DebugEnabled || resp == nil || req == nil {
		return
	}
	logger.LogDebug(c, "antigravity response debug: method=%s url=%s status=%d content_type=%s",
		req.Method,
		req.URL.String(),
		resp.StatusCode,
		resp.Header.Get("Content-Type"),
	)
}

func formatSafeHeaders(headers http.Header) string {
	if len(headers) == 0 {
		return ""
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		values := headers.Values(key)
		if isSensitiveHeader(key) {
			parts = append(parts, key+"=<redacted>")
			continue
		}
		parts = append(parts, key+"="+strings.Join(values, ","))
	}
	return strings.Join(parts, "; ")
}

func formatHeaderOverrideKeys(headerOverride map[string]string) string {
	if len(headerOverride) == 0 {
		return ""
	}
	keys := make([]string, 0, len(headerOverride))
	for key := range headerOverride {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func isSensitiveHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "authorization", "x-api-key", "x-goog-api-key", "cookie", "set-cookie":
		return true
	default:
		return false
	}
}

func filterAntigravityHeaderOverride(headerOverride map[string]string) map[string]string {
	if len(headerOverride) == 0 {
		return headerOverride
	}
	filtered := make(map[string]string, len(headerOverride))
	for key, value := range headerOverride {
		if shouldDropAntigravityHeaderOverride(key) {
			continue
		}
		filtered[key] = value
	}
	return filtered
}

func shouldDropAntigravityHeaderOverride(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return true
	}
	switch normalized {
	case "host",
		"authorization",
		"x-api-key",
		"x-goog-api-key",
		"content-length",
		"accept-encoding",
		"user-agent",
		"anthropic-beta",
		"anthropic-dangerous-direct-browser-access",
		"anthropic-version",
		"x-app",
		"x-stainless-arch",
		"x-stainless-lang",
		"x-stainless-os",
		"x-stainless-package-version",
		"x-stainless-retry-count",
		"x-stainless-runtime",
		"x-stainless-runtime-version",
		"x-stainless-timeout":
		return true
	default:
		return strings.HasPrefix(normalized, "anthropic-") || strings.HasPrefix(normalized, "x-stainless-")
	}
}

func enforceAntigravityHeaders(req *http.Request, info *relaycommon.RelayInfo) {
	if req == nil {
		return
	}
	req.Header.Set("User-Agent", getUserAgent())
	req.Header.Set("Content-Type", "application/json")
	if info != nil && info.IsStream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Del("X-Api-Key")
	req.Header.Del("X-Goog-Api-Key")
	for key := range req.Header {
		if shouldDropAntigravityHeaderOverride(key) && !strings.EqualFold(key, "authorization") && !strings.EqualFold(key, "user-agent") {
			req.Header.Del(key)
		}
	}
}

func buildBaseURLCandidates(primary string) []string {
	seen := make(map[string]struct{}, 4)
	candidates := make([]string, 0, 4)

	appendCandidate := func(baseURL string) {
		baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		if baseURL == "" {
			return
		}
		if _, ok := seen[baseURL]; ok {
			return
		}
		seen[baseURL] = struct{}{}
		candidates = append(candidates, baseURL)
	}

	normalizedPrimary := strings.TrimRight(strings.TrimSpace(primary), "/")
	if normalizedPrimary != "" && !isOfficialAntigravityBaseURL(normalizedPrimary) {
		appendCandidate(normalizedPrimary)
	}
	appendCandidate(antigravityBaseURLDaily)
	appendCandidate(antigravitySandboxBaseURLDaily)
	appendCandidate(antigravityBaseURLProd)
	if isOfficialAntigravityBaseURL(normalizedPrimary) {
		appendCandidate(normalizedPrimary)
	}
	return candidates
}

func isOfficialAntigravityBaseURL(baseURL string) bool {
	switch strings.TrimRight(strings.TrimSpace(baseURL), "/") {
	case antigravityBaseURLDaily, antigravitySandboxBaseURLDaily, antigravityBaseURLProd:
		return true
	default:
		return false
	}
}

func buildRequestURLWithBase(baseURL string, info *relaycommon.RelayInfo) (string, error) {
	action, err := getAntigravityAction(info)
	if err != nil {
		return "", err
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", errors.New("antigravity channel: base url is required")
	}
	requestURL := fmt.Sprintf("%s/v1internal:%s", baseURL, action)
	if info.IsStream {
		requestURL += "?alt=sse"
		if info.RelayMode == relayconstant.RelayModeGemini {
			info.DisablePing = true
		}
	}
	return requestURL, nil
}

func applyRequestHeaderOverride(req *http.Request, headerOverride map[string]string) {
	if req == nil {
		return
	}
	for key, value := range headerOverride {
		req.Header.Set(key, value)
		if strings.EqualFold(key, "host") {
			req.Host = value
		}
	}
}

func parseAntigravitySmartRetryInfo(body []byte) *antigravitySmartRetryInfo {
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil
	}

	errorObj, ok := payload["error"].(map[string]any)
	if !ok {
		return nil
	}

	status, _ := errorObj["status"].(string)
	isResourceExhausted := status == googleRPCStatusResourceExhausted
	isUnavailable := status == googleRPCStatusUnavailable
	if !isResourceExhausted && !isUnavailable {
		return nil
	}

	details, ok := errorObj["details"].([]any)
	if !ok {
		return nil
	}

	var retryDelay time.Duration
	var modelName string
	var hasRateLimitExceeded bool
	var hasModelCapacityExhausted bool

	for _, item := range details {
		detail, ok := item.(map[string]any)
		if !ok {
			continue
		}

		switch detail["@type"] {
		case googleRPCTypeErrorInfo:
			if metadata, ok := detail["metadata"].(map[string]any); ok {
				if model, ok := metadata["model"].(string); ok {
					modelName = strings.TrimSpace(model)
				}
			}
			if reason, ok := detail["reason"].(string); ok {
				switch reason {
				case googleRPCReasonRateLimitExceeded:
					hasRateLimitExceeded = true
				case googleRPCReasonModelCapacityExhausted:
					hasModelCapacityExhausted = true
				}
			}
		case googleRPCTypeRetryInfo:
			if delay, ok := detail["retryDelay"].(string); ok && strings.TrimSpace(delay) != "" {
				if duration, err := time.ParseDuration(delay); err == nil {
					retryDelay = duration
				}
			}
		}
	}

	if isResourceExhausted && !hasRateLimitExceeded {
		return nil
	}
	if isUnavailable && !hasModelCapacityExhausted {
		return nil
	}
	if modelName == "" {
		return nil
	}
	if retryDelay <= 0 {
		retryDelay = antigravityDefaultRateLimitDuration
	}

	return &antigravitySmartRetryInfo{
		RetryDelay:               retryDelay,
		ModelName:                modelName,
		IsModelCapacityExhausted: hasModelCapacityExhausted,
	}
}

func classifySmartRetry(statusCode int, info *antigravitySmartRetryInfo, retryCount int) (time.Duration, bool, bool) {
	if info == nil {
		return 0, false, false
	}

	if statusCode == http.StatusServiceUnavailable && info.IsModelCapacityExhausted {
		if retryCount >= antigravityModelCapacityRetryMaxAttempts {
			return 0, false, true
		}
		return antigravityModelCapacityRetryWait, true, false
	}

	if statusCode == http.StatusTooManyRequests {
		if info.RetryDelay < antigravityRateLimitThreshold && retryCount < antigravitySmartRetryMaxAttempts {
			waitDuration := info.RetryDelay
			if waitDuration < antigravitySmartRetryMinWait {
				waitDuration = antigravitySmartRetryMinWait
			}
			return waitDuration, true, false
		}
		return 0, false, true
	}

	if statusCode == http.StatusServiceUnavailable && retryCount < antigravitySmartRetryMaxAttempts {
		waitDuration := info.RetryDelay
		if waitDuration > antigravityRateLimitThreshold {
			waitDuration = antigravityRateLimitThreshold
		}
		if waitDuration < antigravitySmartRetryMinWait {
			waitDuration = antigravitySmartRetryMinWait
		}
		return waitDuration, true, false
	}

	return 0, false, true
}

func rebuildResponse(resp *http.Response, body []byte) *http.Response {
	if resp == nil {
		return nil
	}
	cloned := new(http.Response)
	*cloned = *resp
	cloned.Header = resp.Header.Clone()
	cloned.Body = io.NopCloser(bytes.NewReader(body))
	cloned.ContentLength = int64(len(body))
	return cloned
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isContextDone(ctx context.Context, err error) bool {
	if ctx == nil {
		return false
	}
	if ctx.Err() != nil {
		return true
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func shouldTryNextBaseURLForNotFound(resp *http.Response, index int, total int) bool {
	if resp == nil || resp.StatusCode != http.StatusNotFound || index >= total-1 {
		return false
	}
	return true
}

func shouldRefreshCredentialForUnauthorized(resp *http.Response, alreadyAttempted bool) bool {
	return !alreadyAttempted && resp != nil && resp.StatusCode == http.StatusUnauthorized
}

func refreshAntigravityCredentialForRetry(c *gin.Context, info *relaycommon.RelayInfo) error {
	if info == nil || info.ChannelId <= 0 {
		return errors.New("antigravity channel: missing channel id for credential refresh")
	}
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	refreshCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	oauthKey, _, err := service.RefreshAntigravityChannelCredential(refreshCtx, info.ChannelId, service.AntigravityCredentialRefreshOptions{ResetCaches: true})
	if err != nil {
		return err
	}
	encoded, err := common.Marshal(oauthKey)
	if err != nil {
		return err
	}
	info.ApiKey = string(encoded)
	if c != nil {
		common.SetContextKey(c, constant.ContextKeyChannelKey, info.ApiKey)
	}
	return nil
}
