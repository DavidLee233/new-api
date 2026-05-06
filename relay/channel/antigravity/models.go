package antigravity

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
)

type fetchAvailableModelsRequest struct {
	Project string `json:"project"`
}

type fetchAvailableModelsResponse struct {
	Models map[string]map[string]any `json:"models"`
}

func FetchAntigravityModels(baseURL, rawKey, proxyURL string) ([]string, error) {
	oauthKey, err := parseOAuthKey(rawKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(oauthKey.AccessToken) == "" {
		return nil, fmt.Errorf("antigravity channel: access_token is required")
	}
	if strings.TrimSpace(oauthKey.ProjectID) == "" {
		return nil, fmt.Errorf("antigravity channel: project_id is required")
	}

	client, err := service.GetHttpClientWithProxy(proxyURL)
	if err != nil {
		return nil, err
	}

	requestPayload, err := common.Marshal(fetchAvailableModelsRequest{
		Project: strings.TrimSpace(oauthKey.ProjectID),
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lastErr error
	for _, candidateBaseURL := range buildBaseURLCandidates(baseURL) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, candidateBaseURL+"/v1internal:fetchAvailableModels", bytes.NewReader(requestPayload))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(oauthKey.AccessToken))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", getUserAgent())

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("fetchAvailableModels failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
			continue
		}

		var result fetchAvailableModelsResponse
		if err = common.Unmarshal(respBody, &result); err != nil {
			lastErr = err
			continue
		}

		models := make([]string, 0, len(result.Models))
		for modelID := range result.Models {
			modelID = strings.TrimSpace(modelID)
			if modelID == "" || shouldSkipAntigravityModel(modelID) {
				continue
			}
			modelID = appconstant.CanonicalClaudeModelAlias(modelID)
			models = append(models, modelID)
		}
		sort.Strings(models)
		return models, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("antigravity channel: base url is required")
}

func shouldSkipAntigravityModel(modelID string) bool {
	switch modelID {
	case "chat_20706", "chat_23310", "tab_flash_lite_preview", "tab_jump_flash_lite_preview", "gemini-2.5-flash-thinking", "gemini-2.5-pro":
		return true
	default:
		return false
	}
}
