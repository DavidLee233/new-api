package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	antigravityOAuthClientIDEnv         = "ANTIGRAVITY_OAUTH_CLIENT_ID"
	antigravityOAuthClientSecretEnv     = "ANTIGRAVITY_OAUTH_CLIENT_SECRET"
	antigravityOAuthUserAgentVersionEnv = "ANTIGRAVITY_USER_AGENT_VERSION"
	antigravityOAuthAuthorizeURL        = "https://accounts.google.com/o/oauth2/v2/auth"
	antigravityOAuthTokenURL            = "https://oauth2.googleapis.com/token"
	antigravityOAuthUserInfoURL         = "https://www.googleapis.com/oauth2/v1/userinfo?alt=json"
	antigravityOAuthRedirectURI         = "http://localhost:8085/callback"
	antigravityOAuthScope               = "https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile https://www.googleapis.com/auth/cclog https://www.googleapis.com/auth/experimentsandconfigs"
	antigravityOAuthProdBaseURL         = "https://cloudcode-pa.googleapis.com"
	antigravityOAuthDailyBaseURL        = "https://daily-cloudcode-pa.googleapis.com"
	antigravityOAuthDailySandboxBaseURL = "https://daily-cloudcode-pa.sandbox.googleapis.com"
	antigravityOAuthLegacyTierID        = "legacy-tier"
	antigravityOAuthAPIClient           = "google-cloud-sdk vscode_cloudshelleditor/0.1"
	antigravityOAuthClientMetadata      = "{\"ideType\":\"IDE_UNSPECIFIED\",\"platform\":\"PLATFORM_UNSPECIFIED\",\"pluginType\":\"GEMINI\"}"
	antigravityOAuthProjectIDMaxRetries = 3
)

var antigravityOAuthBaseURLs = []string{
	antigravityOAuthDailyBaseURL,
	antigravityOAuthDailySandboxBaseURL,
	antigravityOAuthProdBaseURL,
}

type AntigravityOAuthTokenResult struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresAt    time.Time
}

type AntigravityOAuthAuthorizationFlow struct {
	State        string
	Verifier     string
	Challenge    string
	AuthorizeURL string
}

type AntigravityUserInfo struct {
	Email string `json:"email"`
}

func CreateAntigravityOAuthAuthorizationFlow() (*AntigravityOAuthAuthorizationFlow, error) {
	state, err := createStateHex(16)
	if err != nil {
		return nil, err
	}
	verifier, challenge, err := generatePKCEPair()
	if err != nil {
		return nil, err
	}
	authorizeURL, err := buildAntigravityAuthorizeURL(state, challenge)
	if err != nil {
		return nil, err
	}
	return &AntigravityOAuthAuthorizationFlow{
		State:        state,
		Verifier:     verifier,
		Challenge:    challenge,
		AuthorizeURL: authorizeURL,
	}, nil
}

func ExchangeAntigravityAuthorizationCode(ctx context.Context, code string, verifier string) (*AntigravityOAuthTokenResult, error) {
	return ExchangeAntigravityAuthorizationCodeWithProxy(ctx, code, verifier, "")
}

func ExchangeAntigravityAuthorizationCodeWithProxy(ctx context.Context, code string, verifier string, proxyURL string) (*AntigravityOAuthTokenResult, error) {
	client, err := getAntigravityOAuthHTTPClient(proxyURL)
	if err != nil {
		return nil, err
	}
	clientID, err := getAntigravityOAuthClientID()
	if err != nil {
		return nil, err
	}
	clientSecret, err := getAntigravityOAuthClientSecret()
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", strings.TrimSpace(code))
	form.Set("redirect_uri", antigravityOAuthRedirectURI)
	form.Set("grant_type", "authorization_code")
	form.Set("code_verifier", strings.TrimSpace(verifier))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, antigravityOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("antigravity oauth code exchange failed: status=%d", resp.StatusCode)
	}
	if strings.TrimSpace(payload.AccessToken) == "" || strings.TrimSpace(payload.RefreshToken) == "" || payload.ExpiresIn <= 0 {
		return nil, errors.New("antigravity oauth token response missing fields")
	}

	return &AntigravityOAuthTokenResult{
		AccessToken:  strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		TokenType:    strings.TrimSpace(payload.TokenType),
		ExpiresAt:    time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
	}, nil
}

func RefreshAntigravityOAuthToken(ctx context.Context, refreshToken string) (*AntigravityOAuthTokenResult, error) {
	return RefreshAntigravityOAuthTokenWithProxy(ctx, refreshToken, "")
}

func RefreshAntigravityOAuthTokenWithProxy(ctx context.Context, refreshToken string, proxyURL string) (*AntigravityOAuthTokenResult, error) {
	client, err := getAntigravityOAuthHTTPClient(proxyURL)
	if err != nil {
		return nil, err
	}
	clientID, err := getAntigravityOAuthClientID()
	if err != nil {
		return nil, err
	}
	clientSecret, err := getAntigravityOAuthClientSecret()
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("refresh_token", strings.TrimSpace(refreshToken))
	form.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, antigravityOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("antigravity oauth refresh failed: status=%d", resp.StatusCode)
	}
	if strings.TrimSpace(payload.AccessToken) == "" || payload.ExpiresIn <= 0 {
		return nil, errors.New("antigravity oauth refresh response missing fields")
	}

	return &AntigravityOAuthTokenResult{
		AccessToken:  strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		TokenType:    strings.TrimSpace(payload.TokenType),
		ExpiresAt:    time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
	}, nil
}

func FetchAntigravityUserInfoWithProxy(ctx context.Context, accessToken string, proxyURL string) (*AntigravityUserInfo, error) {
	client, err := getAntigravityOAuthHTTPClient(proxyURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, antigravityOAuthUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info AntigravityUserInfo
	if err := common.DecodeJson(resp.Body, &info); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("antigravity userinfo failed: status=%d", resp.StatusCode)
	}
	return &info, nil
}

func FetchAntigravityProjectIDWithProxy(ctx context.Context, accessToken string, proxyURL string) (string, error) {
	client, err := getAntigravityOAuthHTTPClient(proxyURL)
	if err != nil {
		return "", err
	}
	return fetchAntigravityProjectIDWithRetry(ctx, client, accessToken, antigravityOAuthProjectIDMaxRetries)
}

func fetchAntigravityProjectIDWithRetry(ctx context.Context, client *http.Client, accessToken string, maxRetries int) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			if backoff > 8*time.Second {
				backoff = 8 * time.Second
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		projectID, err := loadAntigravityProjectIDFromBaseURLs(ctx, client, accessToken)
		if err == nil && strings.TrimSpace(projectID) != "" {
			return strings.TrimSpace(projectID), nil
		}
		if err != nil {
			lastErr = err
			continue
		}
		lastErr = errors.New("antigravity project_id not found")
	}
	if lastErr != nil {
		return "", fmt.Errorf("failed to fetch antigravity project_id after %d retries: %w", maxRetries, lastErr)
	}
	return "", fmt.Errorf("failed to fetch antigravity project_id after %d retries", maxRetries)
}

func loadAntigravityProjectIDFromBaseURLs(ctx context.Context, client *http.Client, accessToken string) (string, error) {
	var lastErr error
	for _, baseURL := range antigravityOAuthBaseURLs {
		projectID, err := loadAntigravityProjectID(ctx, client, baseURL, accessToken)
		if err == nil && strings.TrimSpace(projectID) != "" {
			return strings.TrimSpace(projectID), nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", errors.New("antigravity project_id not found")
}

func loadAntigravityProjectID(ctx context.Context, client *http.Client, baseURL string, accessToken string) (string, error) {
	payload := map[string]any{
		"metadata": map[string]any{
			"ideType":    "ANTIGRAVITY",
			"platform":   "PLATFORM_UNSPECIFIED",
			"pluginType": "GEMINI",
			"ideVersion": "1.0.0",
			"ideName":    "new-api",
		},
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1internal:loadCodeAssist", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", getAntigravityOAuthUserAgent())
	req.Header.Set("X-Goog-Api-Client", antigravityOAuthAPIClient)
	req.Header.Set("Client-Metadata", antigravityOAuthClientMetadata)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("antigravity loadCodeAssist failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var data map[string]any
	if err := common.Unmarshal(bodyBytes, &data); err != nil {
		return "", err
	}
	if projectID := extractAntigravityProjectID(data); projectID != "" {
		return projectID, nil
	}

	tierID := extractAntigravityTierID(data)
	if tierID == "" {
		return "", errors.New("antigravity project_id not available and tier_id missing")
	}
	return onboardAntigravityUser(ctx, client, accessToken, baseURL, tierID)
}

func onboardAntigravityUser(ctx context.Context, client *http.Client, accessToken string, preferredBaseURL string, tierID string) (string, error) {
	reqPayload := map[string]any{
		"tierId": strings.TrimSpace(tierID),
		"metadata": map[string]any{
			"ideType":    "ANTIGRAVITY",
			"platform":   "PLATFORM_UNSPECIFIED",
			"pluginType": "GEMINI",
		},
	}
	body, err := common.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	var lastErr error
	for _, baseURL := range getAntigravityOAuthBaseURLs(preferredBaseURL) {
		for attempt := 0; attempt < 5; attempt++ {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1internal:onboardUser", strings.NewReader(string(body)))
			if err != nil {
				return "", err
			}
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", getAntigravityOAuthUserAgent())
			req.Header.Set("X-Goog-Api-Client", antigravityOAuthAPIClient)
			req.Header.Set("Client-Metadata", antigravityOAuthClientMetadata)

			resp, err := client.Do(req)
			if err != nil {
				lastErr = err
				break
			}

			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				return "", readErr
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				lastErr = fmt.Errorf("antigravity onboardUser failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
				break
			}

			var data map[string]any
			if err := common.Unmarshal(bodyBytes, &data); err != nil {
				return "", err
			}

			done, _ := data["done"].(bool)
			if done {
				if responseMap, ok := data["response"].(map[string]any); ok {
					if projectID := extractAntigravityProjectID(responseMap); projectID != "" {
						return projectID, nil
					}
				}
				return "", errors.New("antigravity onboarding completed without project_id")
			}

			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", errors.New("antigravity onboarding did not return project_id")
}

func getAntigravityOAuthBaseURLs(preferredBaseURL string) []string {
	ordered := make([]string, 0, len(antigravityOAuthBaseURLs))
	seen := make(map[string]struct{}, len(antigravityOAuthBaseURLs))

	preferredBaseURL = strings.TrimSpace(preferredBaseURL)
	if preferredBaseURL != "" {
		ordered = append(ordered, preferredBaseURL)
		seen[preferredBaseURL] = struct{}{}
	}

	for _, baseURL := range antigravityOAuthBaseURLs {
		baseURL = strings.TrimSpace(baseURL)
		if baseURL == "" {
			continue
		}
		if _, ok := seen[baseURL]; ok {
			continue
		}
		ordered = append(ordered, baseURL)
		seen[baseURL] = struct{}{}
	}
	return ordered
}

func extractAntigravityProjectID(data map[string]any) string {
	if data == nil {
		return ""
	}
	raw, ok := data["cloudaicompanionProject"]
	if !ok || raw == nil {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		if id, ok := value["id"]; ok {
			return strings.TrimSpace(fmt.Sprintf("%v", id))
		}
	}
	return ""
}

func extractAntigravityTierID(data map[string]any) string {
	if data == nil {
		return ""
	}
	if tierID := extractAntigravityAllowedTierID(data["allowedTiers"], true); tierID != "" {
		return tierID
	}
	if tierID := extractAntigravityTierIDValue(data["paidTier"]); tierID != "" {
		return tierID
	}
	if tierID := extractAntigravityTierIDValue(data["currentTier"]); tierID != "" {
		return tierID
	}
	if tierID := extractAntigravityAllowedTierID(data["allowedTiers"], false); tierID != "" {
		return tierID
	}
	return antigravityOAuthLegacyTierID
}

func extractAntigravityTierIDValue(raw any) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		if id, ok := value["id"]; ok {
			return strings.TrimSpace(fmt.Sprintf("%v", id))
		}
	}
	return ""
}

func extractAntigravityAllowedTierID(raw any, onlyDefault bool) string {
	tiers, ok := raw.([]any)
	if !ok {
		return ""
	}

	firstTierID := ""
	for _, rawTier := range tiers {
		tier, ok := rawTier.(map[string]any)
		if !ok {
			continue
		}

		tierID := strings.TrimSpace(fmt.Sprintf("%v", tier["id"]))
		if tierID == "" {
			continue
		}
		if firstTierID == "" {
			firstTierID = tierID
		}

		isDefault, _ := tier["isDefault"].(bool)
		if isDefault {
			return tierID
		}
	}

	if onlyDefault {
		return ""
	}
	return firstTierID
}

func getAntigravityOAuthClientID() (string, error) {
	clientID := strings.TrimSpace(common.GetEnvOrDefaultString(antigravityOAuthClientIDEnv, ""))
	if clientID == "" {
		return "", fmt.Errorf("antigravity oauth client_id is not configured, please set %s", antigravityOAuthClientIDEnv)
	}
	return clientID, nil
}

func getAntigravityOAuthClientSecret() (string, error) {
	clientSecret := strings.TrimSpace(common.GetEnvOrDefaultString(antigravityOAuthClientSecretEnv, ""))
	if clientSecret == "" {
		return "", fmt.Errorf("antigravity oauth client_secret is not configured, please set %s", antigravityOAuthClientSecretEnv)
	}
	return clientSecret, nil
}

func getAntigravityOAuthUserAgent() string {
	version := common.GetEnvOrDefaultString(antigravityOAuthUserAgentVersionEnv, "1.20.5")
	return fmt.Sprintf("antigravity/%s windows/amd64", version)
}

func getAntigravityOAuthHTTPClient(proxyURL string) (*http.Client, error) {
	baseClient, err := GetHttpClientWithProxy(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, err
	}
	if baseClient == nil {
		return &http.Client{Timeout: defaultHTTPTimeout}, nil
	}
	clientCopy := *baseClient
	clientCopy.Timeout = defaultHTTPTimeout
	return &clientCopy, nil
}

func buildAntigravityAuthorizeURL(state string, challenge string) (string, error) {
	clientID, err := getAntigravityOAuthClientID()
	if err != nil {
		return "", err
	}
	u, err := url.Parse(antigravityOAuthAuthorizeURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", clientID)
	q.Set("redirect_uri", antigravityOAuthRedirectURI)
	q.Set("response_type", "code")
	q.Set("scope", antigravityOAuthScope)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	q.Set("include_granted_scopes", "true")
	u.RawQuery = q.Encode()
	return u.String(), nil
}
