package antigravity

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

func TestParseAntigravitySmartRetryInfoRateLimit(t *testing.T) {
	body := []byte(`{
		"error": {
			"status": "RESOURCE_EXHAUSTED",
			"details": [
				{
					"@type": "type.googleapis.com/google.rpc.ErrorInfo",
					"reason": "RATE_LIMIT_EXCEEDED",
					"metadata": {
						"model": "gemini-2.5-pro"
					}
				},
				{
					"@type": "type.googleapis.com/google.rpc.RetryInfo",
					"retryDelay": "2.5s"
				}
			]
		}
	}`)

	info := parseAntigravitySmartRetryInfo(body)
	if info == nil {
		t.Fatal("expected retry info, got nil")
	}
	if info.ModelName != "gemini-2.5-pro" {
		t.Fatalf("unexpected model name: %s", info.ModelName)
	}
	if info.RetryDelay != 2500*time.Millisecond {
		t.Fatalf("unexpected retry delay: %v", info.RetryDelay)
	}
	if info.IsModelCapacityExhausted {
		t.Fatal("rate limit case should not be model capacity exhausted")
	}
}

func TestParseAntigravitySmartRetryInfoModelCapacity(t *testing.T) {
	body := []byte(`{
		"error": {
			"status": "UNAVAILABLE",
			"details": [
				{
					"@type": "type.googleapis.com/google.rpc.ErrorInfo",
					"reason": "MODEL_CAPACITY_EXHAUSTED",
					"metadata": {
						"model": "gemini-3-pro-image"
					}
				}
			]
		}
	}`)

	info := parseAntigravitySmartRetryInfo(body)
	if info == nil {
		t.Fatal("expected retry info, got nil")
	}
	if info.ModelName != "gemini-3-pro-image" {
		t.Fatalf("unexpected model name: %s", info.ModelName)
	}
	if info.RetryDelay != antigravityDefaultRateLimitDuration {
		t.Fatalf("unexpected default retry delay: %v", info.RetryDelay)
	}
	if !info.IsModelCapacityExhausted {
		t.Fatal("expected model capacity exhausted")
	}
}

func TestBuildAntigravityImageRequest(t *testing.T) {
	n := uint(3)
	request, err := buildAntigravityImageRequest(dto.ImageRequest{
		Prompt:  "draw a lighthouse",
		Size:    "1792x1024",
		Quality: "hd",
		N:       &n,
	})
	if err != nil {
		t.Fatalf("buildAntigravityImageRequest returned error: %v", err)
	}
	if len(request.Contents) != 1 || len(request.Contents[0].Parts) != 1 {
		t.Fatalf("unexpected contents shape: %+v", request.Contents)
	}
	if got := request.Contents[0].Parts[0].Text; got != "draw a lighthouse" {
		t.Fatalf("unexpected prompt: %s", got)
	}
	if len(request.GenerationConfig.ResponseModalities) != 2 {
		t.Fatalf("unexpected modalities: %+v", request.GenerationConfig.ResponseModalities)
	}
	if string(request.GenerationConfig.ImageConfig) != `{"aspectRatio":"16:9","imageSize":"4K"}` {
		t.Fatalf("unexpected image config: %s", string(request.GenerationConfig.ImageConfig))
	}
	if request.GenerationConfig.CandidateCount == nil || *request.GenerationConfig.CandidateCount != 1 {
		t.Fatalf("unexpected candidate count: %+v", request.GenerationConfig.CandidateCount)
	}
}

func TestParseAntigravityErrorInfo(t *testing.T) {
	body := []byte(`{
		"error": {
			"code": 429,
			"message": "Quota temporarily exhausted",
			"status": "RESOURCE_EXHAUSTED",
			"details": [
				{
					"@type": "type.googleapis.com/google.rpc.ErrorInfo",
					"reason": "RATE_LIMIT_EXCEEDED",
					"metadata": {
						"model": "gemini-2.5-pro"
					}
				},
				{
					"@type": "type.googleapis.com/google.rpc.RetryInfo",
					"retryDelay": "3s"
				}
			]
		}
	}`)

	info := parseAntigravityErrorInfo(body)
	if info == nil {
		t.Fatal("expected error info, got nil")
	}
	if !info.IsRateLimitExceeded {
		t.Fatal("expected rate limit exceeded")
	}
	if info.ModelName != "gemini-2.5-pro" {
		t.Fatalf("unexpected model: %s", info.ModelName)
	}
	if info.RetryDelay != 3*time.Second {
		t.Fatalf("unexpected retry delay: %v", info.RetryDelay)
	}
}

func TestMapAntigravityErrorModelNotFound(t *testing.T) {
	openAIError, errorCode := mapAntigravityError(&antigravityErrorInfo{
		UpstreamStatus: "NOT_FOUND",
		ModelName:      "gemini-missing",
	}, 404)

	if errorCode != types.ErrorCodeModelNotFound {
		t.Fatalf("unexpected error code: %s", errorCode)
	}
	if openAIError.Type != "invalid_request_error" {
		t.Fatalf("unexpected error type: %s", openAIError.Type)
	}
	if openAIError.Message != "Antigravity model not found: gemini-missing" {
		t.Fatalf("unexpected message: %s", openAIError.Message)
	}
}

func TestMapAntigravityErrorModelCapacity(t *testing.T) {
	openAIError, errorCode := mapAntigravityError(&antigravityErrorInfo{
		UpstreamStatus:           "UNAVAILABLE",
		ModelName:                "gemini-3-pro-image",
		RetryDelay:               10 * time.Second,
		IsModelCapacityExhausted: true,
	}, 503)

	if errorCode != types.ErrorCodeBadResponseStatusCode {
		t.Fatalf("unexpected error code: %s", errorCode)
	}
	if openAIError.Type != "overloaded_error" {
		t.Fatalf("unexpected error type: %s", openAIError.Type)
	}
	if openAIError.Message != "Antigravity model capacity exhausted for gemini-3-pro-image, retry after 10s" {
		t.Fatalf("unexpected message: %s", openAIError.Message)
	}
}

func TestBuildBaseURLCandidatesPrefersDaily(t *testing.T) {
	candidates := buildBaseURLCandidates("")
	if len(candidates) < 3 {
		t.Fatalf("unexpected candidate count: %d", len(candidates))
	}
	if candidates[0] != antigravityBaseURLDaily {
		t.Fatalf("expected first candidate to be daily, got %s", candidates[0])
	}
	if candidates[1] != antigravitySandboxBaseURLDaily {
		t.Fatalf("expected second candidate to be daily sandbox, got %s", candidates[1])
	}
	if candidates[2] != antigravityBaseURLProd {
		t.Fatalf("expected third candidate to be prod, got %s", candidates[2])
	}
}

func TestBuildBaseURLCandidatesPrefersDailyOverOfficialPrimary(t *testing.T) {
	candidates := buildBaseURLCandidates(antigravityBaseURLProd)
	if len(candidates) < 3 {
		t.Fatalf("unexpected candidate count: %d", len(candidates))
	}
	if candidates[0] != antigravityBaseURLDaily {
		t.Fatalf("expected first candidate to be daily, got %s", candidates[0])
	}
	if candidates[1] != antigravitySandboxBaseURLDaily {
		t.Fatalf("expected second candidate to be daily sandbox, got %s", candidates[1])
	}
	if candidates[2] != antigravityBaseURLProd {
		t.Fatalf("expected third candidate to be prod, got %s", candidates[2])
	}
}

func TestBuildBaseURLCandidatesKeepsCustomPrimaryFirst(t *testing.T) {
	customBaseURL := "https://example-antigravity-proxy.invalid"
	candidates := buildBaseURLCandidates(customBaseURL)
	if len(candidates) < 4 {
		t.Fatalf("unexpected candidate count: %d", len(candidates))
	}
	if candidates[0] != customBaseURL {
		t.Fatalf("expected custom primary first, got %s", candidates[0])
	}
	if candidates[1] != antigravityBaseURLDaily {
		t.Fatalf("expected daily second, got %s", candidates[1])
	}
}

func TestShouldSkipAntigravityModel(t *testing.T) {
	for _, modelID := range []string{"chat_20706", "tab_flash_lite_preview", "gemini-2.5-pro"} {
		if !shouldSkipAntigravityModel(modelID) {
			t.Fatalf("expected model %s to be filtered", modelID)
		}
	}
	if shouldSkipAntigravityModel("claude-sonnet-4-6") {
		t.Fatal("claude-sonnet-4-6 should not be filtered")
	}
}

func TestWrapV1InternalRequestUsesMappedModel(t *testing.T) {
	wrapped, err := wrapV1InternalRequest("project-123", "claude-sonnet-4-6", []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
	if err != nil {
		t.Fatalf("wrapV1InternalRequest returned error: %v", err)
	}

	body, err := common.Marshal(wrapped)
	if err != nil {
		t.Fatalf("marshal wrapped request: %v", err)
	}
	var decoded map[string]any
	if err := common.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal wrapped request: %v", err)
	}
	if decoded["model"] != "claude-sonnet-4-6" {
		t.Fatalf("unexpected wrapper model: %v", decoded["model"])
	}
}

func TestCleanGeminiRequestAddsEmptyToolParameters(t *testing.T) {
	body := []byte(`{
		"contents": [{"role": "user", "parts": [{"text": "hi"}]}],
		"tools": [{
			"functionDeclarations": [
				{"name": "CronList", "description": "list cron jobs"},
				{"name": "Bash", "parameters": {"properties": {"command": {"type": "string"}}}}
			]
		}]
	}`)

	cleaned, err := cleanGeminiRequest(body)
	if err != nil {
		t.Fatalf("cleanGeminiRequest returned error: %v", err)
	}

	var decoded map[string]any
	if err := common.Unmarshal(cleaned, &decoded); err != nil {
		t.Fatalf("unmarshal cleaned request: %v", err)
	}
	tools := decoded["tools"].([]any)
	tool := tools[0].(map[string]any)
	functions := tool["functionDeclarations"].([]any)
	first := functions[0].(map[string]any)
	parameters := first["parameters"].(map[string]any)
	if parameters["type"] != "object" {
		t.Fatalf("expected empty tool parameters to default to object, got: %v", parameters)
	}
}

func TestCleanGeminiRequestNormalizesToolParametersToJSONSchema(t *testing.T) {
	body := []byte(`{
		"contents": [{"role": "user", "parts": [{"text": "hi"}]}],
		"tools": [{
			"functionDeclarations": [{
				"name": "CustomTool",
				"parameters": {
					"type": "OBJECT",
					"nullable": true,
					"description": ["bad"],
					"anyOf": ["bad"],
					"propertyOrdering": ["path"],
					"required": ["path", "missing", ""],
					"properties": {
						"path": {"type": "STRING", "nullable": true},
						"items": {"type": "ARRAY", "items": [{"type": "INTEGER"}, {"type": "STRING"}]},
						"choice": {"enum": ["a", "b"], "default": "a", "title": "Choice"}
					}
				}
			}]
		}]
	}`)

	cleaned, err := cleanGeminiRequest(body)
	if err != nil {
		t.Fatalf("cleanGeminiRequest returned error: %v", err)
	}

	var decoded map[string]any
	if err := common.Unmarshal(cleaned, &decoded); err != nil {
		t.Fatalf("unmarshal cleaned request: %v", err)
	}
	functions := decoded["tools"].([]any)[0].(map[string]any)["functionDeclarations"].([]any)
	parameters := functions[0].(map[string]any)["parameters"].(map[string]any)
	if _, ok := parameters["propertyOrdering"]; ok {
		t.Fatalf("propertyOrdering should be removed: %+v", parameters)
	}
	if _, ok := parameters["description"]; ok {
		t.Fatalf("non-string description should be removed: %+v", parameters)
	}
	if _, ok := parameters["anyOf"]; ok {
		t.Fatalf("degenerate anyOf should be removed from object schema: %+v", parameters)
	}
	required := parameters["required"].([]any)
	if len(required) != 1 || required[0] != "path" {
		t.Fatalf("unexpected required list: %+v", required)
	}
	path := parameters["properties"].(map[string]any)["path"].(map[string]any)
	if path["type"] != "string" {
		t.Fatalf("unexpected path type: %+v", path)
	}
	items := parameters["properties"].(map[string]any)["items"].(map[string]any)
	if items["type"] != "array" {
		t.Fatalf("unexpected array type: %+v", items)
	}
	itemSchema := items["items"].(map[string]any)
	if itemSchema["type"] != "integer" {
		t.Fatalf("tuple-style items should keep first schema, got %+v", itemSchema)
	}
	choice := parameters["properties"].(map[string]any)["choice"].(map[string]any)
	if choice["type"] != "string" {
		t.Fatalf("enum string type should be inferred, got %+v", choice)
	}
	if _, ok := choice["default"]; ok {
		t.Fatalf("default should be removed from strict tool schema: %+v", choice)
	}
	if _, ok := choice["title"]; ok {
		t.Fatalf("title should be removed from strict tool schema: %+v", choice)
	}
}

func TestExtractAntigravityInvalidToolSchemaIndex(t *testing.T) {
	body := []byte(`{"error":{"message":"tools.24.custom.input_schema: JSON schema is invalid. It must match JSON Schema draft 2020-12"}}`)

	index, ok := extractAntigravityInvalidToolSchemaIndex(body)
	if !ok {
		t.Fatal("expected invalid tool schema index")
	}
	if index != 24 {
		t.Fatalf("unexpected index: %d", index)
	}
}

func TestDowngradeAntigravityToolSchema(t *testing.T) {
	body := []byte(`{
		"request": {
			"tools": [{
				"functionDeclarations": [
					{"name": "Bash", "parameters": {"type": "object", "properties": {"command": {"type": "string"}}}},
					{"name": "TaskUpdate", "parameters": {"type": "object", "properties": {"bad": {"anyOf": ["bad"]}}}}
				]
			}]
		}
	}`)

	downgraded, toolName, ok, err := downgradeAntigravityToolSchema(body, 1)
	if err != nil {
		t.Fatalf("downgradeAntigravityToolSchema returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected tool schema to be downgraded")
	}
	if toolName != "TaskUpdate" {
		t.Fatalf("unexpected tool name: %s", toolName)
	}

	var decoded map[string]any
	if err := common.Unmarshal(downgraded, &decoded); err != nil {
		t.Fatalf("unmarshal downgraded body: %v", err)
	}
	functions := decoded["request"].(map[string]any)["tools"].([]any)[0].(map[string]any)["functionDeclarations"].([]any)
	firstParams := functions[0].(map[string]any)["parameters"].(map[string]any)
	if _, ok := firstParams["properties"]; !ok {
		t.Fatalf("first tool schema should be untouched, got %+v", firstParams)
	}
	secondParams := functions[1].(map[string]any)["parameters"].(map[string]any)
	if secondParams["type"] != "object" {
		t.Fatalf("downgraded schema should be object, got %+v", secondParams)
	}
	if len(secondParams["properties"].(map[string]any)) != 0 {
		t.Fatalf("downgraded schema properties should be empty, got %+v", secondParams)
	}
}

func TestGetAntigravityAction_AllowsClaudeRelayFormatWithoutRelayMode(t *testing.T) {
	action, err := getAntigravityAction(&relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		IsStream:    true,
	})
	if err != nil {
		t.Fatalf("getAntigravityAction returned error: %v", err)
	}
	if action != "streamGenerateContent" {
		t.Fatalf("unexpected action: %s", action)
	}
}

func TestGetAntigravityAction_RejectsUnknownRelayFormatWithoutRelayMode(t *testing.T) {
	_, err := getAntigravityAction(&relaycommon.RelayInfo{})
	if err == nil {
		t.Fatal("expected unsupported relay mode error")
	}
}
