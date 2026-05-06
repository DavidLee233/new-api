package antigravity

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

const (
	defaultUserAgentVersion = "1.20.5"
	userAgentVersionEnv     = "ANTIGRAVITY_USER_AGENT_VERSION"
	identityPatchText       = "You are Antigravity, a powerful agentic AI coding assistant designed by the Google Deepmind team working on Advanced Agentic Coding."
)

type oauthKey struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ProjectID    string `json:"project_id,omitempty"`
}

func parseOAuthKey(raw string) (*oauthKey, error) {
	keyText := strings.TrimSpace(raw)
	if keyText == "" {
		return nil, errors.New("antigravity channel: empty oauth key")
	}
	if !strings.HasPrefix(keyText, "{") {
		return nil, errors.New("antigravity channel: key must be a JSON object")
	}

	var key oauthKey
	if err := common.Unmarshal([]byte(keyText), &key); err != nil {
		return nil, errors.New("antigravity channel: invalid oauth key json")
	}
	return &key, nil
}

func getUserAgent() string {
	version := common.GetEnvOrDefaultString(userAgentVersionEnv, defaultUserAgentVersion)
	return fmt.Sprintf("antigravity/%s windows/amd64", version)
}

func getAntigravityAction(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("antigravity channel: relay info is required")
	}

	if info.RelayMode == 0 {
		switch info.RelayFormat {
		case types.RelayFormatOpenAI, types.RelayFormatClaude, types.RelayFormatGemini, types.RelayFormatOpenAIImage:
			// Claude-compatible /v1/messages currently reaches Antigravity with RelayModeUnknown.
			// Antigravity only needs to know whether the request is stream/non-stream here.
		default:
			return "", errors.New("antigravity channel: unsupported relay mode")
		}
	}

	if info.IsStream {
		return "streamGenerateContent", nil
	}
	return "generateContent", nil
}

func wrapV1InternalRequest(projectID, model string, originalBody []byte) (map[string]any, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("antigravity channel: project_id is required")
	}
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("antigravity channel: upstream model is required")
	}

	var request any
	if err := common.Unmarshal(originalBody, &request); err != nil {
		return nil, fmt.Errorf("antigravity channel: invalid gemini payload: %w", err)
	}

	return map[string]any{
		"project":     projectID,
		"requestId":   "agent-" + uuid.NewString(),
		"userAgent":   "antigravity",
		"requestType": "agent",
		"model":       model,
		"request":     request,
	}, nil
}

func unwrapV1InternalResponse(body []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return trimmed, nil
	}
	if !gjson.ValidBytes(trimmed) {
		return trimmed, nil
	}

	responseField := gjson.GetBytes(trimmed, "response")
	if !responseField.Exists() {
		return trimmed, nil
	}
	return []byte(responseField.Raw), nil
}

func injectIdentityPatch(body []byte) ([]byte, error) {
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	systemInstruction := getInstructionMap(payload, "systemInstruction", "system_instruction")
	if containsIdentityPatch(systemInstruction) {
		return body, nil
	}

	if systemInstruction == nil {
		systemInstruction = map[string]any{
			"parts": []any{},
		}
	}

	parts, _ := systemInstruction["parts"].([]any)
	systemInstruction["parts"] = append([]any{map[string]any{"text": identityPatchText}}, parts...)
	payload["systemInstruction"] = systemInstruction
	delete(payload, "system_instruction")

	return common.Marshal(payload)
}

func containsIdentityPatch(systemInstruction map[string]any) bool {
	if systemInstruction == nil {
		return false
	}

	parts, ok := systemInstruction["parts"].([]any)
	if !ok {
		return false
	}
	for _, part := range parts {
		partMap, ok := part.(map[string]any)
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprintf("%v", partMap["text"]))
		if strings.Contains(text, "You are Antigravity") {
			return true
		}
	}
	return false
}

func getInstructionMap(payload map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		if instruction, ok := value.(map[string]any); ok {
			return instruction
		}
	}
	return nil
}

func cleanGeminiRequest(body []byte) ([]byte, error) {
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	modified := false

	if contents, ok := payload["contents"].([]any); ok {
		filtered := make([]any, 0, len(contents))
		for _, item := range contents {
			contentMap, ok := item.(map[string]any)
			if !ok {
				filtered = append(filtered, item)
				continue
			}
			parts, ok := contentMap["parts"].([]any)
			if !ok || len(parts) == 0 {
				modified = true
				continue
			}
			filtered = append(filtered, item)
		}
		payload["contents"] = filtered
	}

	if systemInstruction := getInstructionMap(payload, "systemInstruction", "system_instruction"); systemInstruction != nil {
		if parts, ok := systemInstruction["parts"].([]any); ok && len(parts) == 0 {
			delete(payload, "systemInstruction")
			delete(payload, "system_instruction")
			modified = true
		}
	}

	if tools, ok := payload["tools"].([]any); ok {
		for _, tool := range tools {
			toolMap, ok := tool.(map[string]any)
			if !ok {
				continue
			}
			functions, ok := toolMap["functionDeclarations"].([]any)
			if !ok {
				functions, _ = toolMap["function_declarations"].([]any)
			}
			for _, fn := range functions {
				fnMap, ok := fn.(map[string]any)
				if !ok {
					continue
				}
				parameters, ok := fnMap["parameters"].(map[string]any)
				if !ok {
					fnMap["parameters"] = emptyAntigravityToolParameters()
					modified = true
					continue
				}
				if strings.TrimSpace(fmt.Sprintf("%v", parameters["type"])) == "" {
					parameters["type"] = "object"
					modified = true
				}
				fnMap["parameters"] = cleanSchemaValue(parameters)
				modified = true
			}
		}
	}

	if !modified {
		return body, nil
	}
	return common.Marshal(payload)
}

func emptyAntigravityToolParameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func cleanSchemaValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(typed))
		for key, item := range typed {
			switch key {
			case "properties":
				if properties, ok := item.(map[string]any); ok {
					cleanedProperties := make(map[string]any, len(properties))
					for propertyName, propertyValue := range properties {
						cleanedProperties[propertyName] = cleanSchemaValue(propertyValue)
					}
					cleaned[key] = cleanedProperties
				}
			case "type", "description", "enum", "required", "items", "anyOf":
				cleaned[key] = cleanSchemaValue(item)
			}
		}
		normalizeAntigravityJSONSchema(cleaned)
		if properties, ok := cleaned["properties"].(map[string]any); ok && len(properties) == 0 {
			delete(cleaned, "properties")
		}
		if required, ok := cleaned["required"].([]any); ok && len(required) == 0 {
			delete(cleaned, "required")
		}
		return cleaned
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, cleanSchemaValue(item))
		}
		return items
	default:
		return value
	}
}

func normalizeAntigravityJSONSchema(schema map[string]any) {
	if len(schema) == 0 {
		return
	}

	normalizeAntigravitySchemaType(schema)
	normalizeAntigravityDescription(schema)
	normalizeAntigravityEnum(schema)

	if properties, ok := schema["properties"].(map[string]any); ok {
		for name, property := range properties {
			switch propertyMap := property.(type) {
			case map[string]any:
				normalizeAntigravityJSONSchema(propertyMap)
				properties[name] = propertyMap
			case bool:
				properties[name] = schemaForBooleanJSONSchema(propertyMap)
			default:
				properties[name] = map[string]any{"type": "string"}
			}
		}
	}

	if items, ok := schema["items"].([]any); ok {
		if len(items) == 0 {
			delete(schema, "items")
		} else {
			schema["items"] = cleanSchemaValue(items[0])
		}
	} else if itemMap, ok := schema["items"].(map[string]any); ok {
		schema["items"] = cleanSchemaValue(itemMap)
	} else if itemBool, ok := schema["items"].(bool); ok {
		schema["items"] = schemaForBooleanJSONSchema(itemBool)
	} else if _, ok := schema["items"]; ok {
		delete(schema, "items")
	}

	if nested, ok := schema["anyOf"].([]any); ok {
		filtered := make([]any, 0, len(nested))
		for _, item := range nested {
			cleaned := cleanSchemaValue(item)
			if cleanedMap, ok := cleaned.(map[string]any); ok {
				normalizeAntigravityJSONSchema(cleanedMap)
				if len(cleanedMap) == 0 {
					continue
				}
			}
			if cleanedBool, ok := cleaned.(bool); ok {
				cleaned = schemaForBooleanJSONSchema(cleanedBool)
			}
			if _, ok := cleaned.(map[string]any); !ok {
				cleaned = map[string]any{"type": "string"}
			}
			filtered = append(filtered, cleaned)
		}
		if len(filtered) == 0 {
			delete(schema, "anyOf")
		} else if len(filtered) == 1 {
			if only, ok := filtered[0].(map[string]any); ok {
				delete(schema, "anyOf")
				for key, value := range only {
					if _, exists := schema[key]; !exists {
						schema[key] = value
					}
				}
			} else {
				schema["anyOf"] = filtered
			}
		} else {
			schema["anyOf"] = filtered
		}
	} else if _, ok := schema["anyOf"]; ok {
		delete(schema, "anyOf")
	}

	normalizeRequiredFields(schema)
	ensureAntigravitySchemaType(schema)
}

func normalizeAntigravitySchemaType(schema map[string]any) {
	rawType, hasType := schema["type"]
	nullable := false
	if value, ok := schema["nullable"].(bool); ok && value {
		nullable = true
	}
	delete(schema, "nullable")

	switch typed := rawType.(type) {
	case string:
		normalized := normalizeJSONSchemaTypeName(typed)
		if normalized == "" {
			delete(schema, "type")
			return
		}
		schema["type"] = normalized
	case []any:
		var chosen string
		for _, item := range typed {
			itemType := normalizeJSONSchemaTypeName(common.Interface2String(item))
			if itemType == "" || itemType == "null" {
				continue
			}
			chosen = itemType
			break
		}
		if chosen == "" {
			delete(schema, "type")
		} else {
			schema["type"] = chosen
		}
	default:
		if hasType {
			delete(schema, "type")
		}
	}
	_ = nullable

	if _, ok := schema["type"]; !ok {
		if _, hasProperties := schema["properties"]; hasProperties {
			schema["type"] = "object"
		} else if _, hasItems := schema["items"]; hasItems {
			schema["type"] = "array"
		}
	}
}

func normalizeAntigravityDescription(schema map[string]any) {
	rawDescription, ok := schema["description"]
	if !ok {
		return
	}
	description, ok := rawDescription.(string)
	if !ok {
		delete(schema, "description")
		return
	}
	description = strings.TrimSpace(description)
	if description == "" {
		delete(schema, "description")
		return
	}
	schema["description"] = description
}

func normalizeAntigravityEnum(schema map[string]any) {
	rawEnum, ok := schema["enum"]
	if !ok {
		return
	}
	enumValues, ok := rawEnum.([]any)
	if !ok || len(enumValues) == 0 {
		delete(schema, "enum")
		return
	}
	schema["enum"] = enumValues
	if _, hasType := schema["type"]; !hasType {
		for _, value := range enumValues {
			if inferredType := inferJSONSchemaType(value); inferredType != "" {
				schema["type"] = inferredType
				return
			}
		}
	}
}

func inferJSONSchemaType(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64, float32, int, int64, int32, uint, uint64, uint32:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return ""
	}
}

func normalizeJSONSchemaTypeName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "object":
		return "object"
	case "array":
		return "array"
	case "string":
		return "string"
	case "integer":
		return "integer"
	case "number":
		return "number"
	case "boolean":
		return "boolean"
	case "null":
		return "null"
	default:
		return ""
	}
}

func normalizeRequiredFields(schema map[string]any) {
	rawRequired, ok := schema["required"]
	if !ok {
		return
	}

	var values []any
	switch typed := rawRequired.(type) {
	case []any:
		values = typed
	case []string:
		values = make([]any, 0, len(typed))
		for _, item := range typed {
			values = append(values, item)
		}
	default:
		delete(schema, "required")
		return
	}

	allowedProperties := map[string]struct{}{}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for propertyName := range properties {
			allowedProperties[propertyName] = struct{}{}
		}
	}

	normalized := make([]any, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, item := range values {
		name := strings.TrimSpace(common.Interface2String(item))
		if name == "" {
			continue
		}
		if len(allowedProperties) > 0 {
			if _, ok := allowedProperties[name]; !ok {
				continue
			}
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	if len(normalized) == 0 {
		delete(schema, "required")
		return
	}
	schema["required"] = normalized
}

func ensureAntigravitySchemaType(schema map[string]any) {
	if _, ok := schema["type"]; ok {
		return
	}
	if _, ok := schema["anyOf"]; ok {
		return
	}
	if _, ok := schema["enum"]; ok {
		return
	}
	if _, ok := schema["properties"]; ok {
		schema["type"] = "object"
		return
	}
	if _, ok := schema["items"]; ok {
		schema["type"] = "array"
		return
	}
	if len(schema) > 0 {
		schema["type"] = "string"
	}
}

func schemaForBooleanJSONSchema(value bool) map[string]any {
	return map[string]any{"type": "string"}
}

func newStreamUnwrapper(body io.ReadCloser) io.ReadCloser {
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		defer body.Close()
		defer pipeWriter.Close()

		reader := bufio.NewReader(body)
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				rewritten, rewriteErr := rewriteSSELine(line)
				if rewriteErr != nil {
					pipeWriter.CloseWithError(rewriteErr)
					return
				}
				if len(rewritten) > 0 {
					if _, writeErr := pipeWriter.Write(rewritten); writeErr != nil {
						return
					}
				}
			}

			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				pipeWriter.CloseWithError(err)
				return
			}
		}
	}()

	return pipeReader
}

func rewriteSSELine(line []byte) ([]byte, error) {
	trimmed := bytes.TrimRight(line, "\r\n")
	if len(trimmed) == 0 {
		return line, nil
	}
	if !bytes.HasPrefix(trimmed, []byte("data:")) {
		return line, nil
	}

	payload := bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("data:")))
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return line, nil
	}

	unwrapped, err := unwrapV1InternalResponse(payload)
	if err != nil {
		return nil, err
	}

	suffix := line[len(trimmed):]
	if len(suffix) == 0 {
		suffix = []byte("\n")
	}
	rewritten := append([]byte("data: "), unwrapped...)
	rewritten = append(rewritten, suffix...)
	return rewritten, nil
}
