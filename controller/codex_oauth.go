package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type codexOAuthCompleteRequest struct {
	Input  string `json:"input"`
	Append bool   `json:"append,omitempty"`
}

type codexOAuthImportRequest struct {
	Input  string `json:"input"`
	Append bool   `json:"append,omitempty"`
}

type codexOAuthImportEnvelope struct {
	Type     string                      `json:"type"`
	Accounts []codexOAuthImportAccount   `json:"accounts"`
}

type codexOAuthImportAccount struct {
	Credentials codexOAuthImportKey `json:"credentials"`
}

type codexOAuthImportKey struct {
	codex.OAuthKey
	ChatGPTAccountID string `json:"chatgpt_account_id,omitempty"`
}

func codexOAuthSessionKey(channelID int, field string) string {
	return fmt.Sprintf("codex_oauth_%s_%d", field, channelID)
}

func parseCodexAuthorizationInput(input string) (code string, state string, err error) {
	v := strings.TrimSpace(input)
	if v == "" {
		return "", "", errors.New("empty input")
	}
	if strings.Contains(v, "#") {
		parts := strings.SplitN(v, "#", 2)
		code = strings.TrimSpace(parts[0])
		state = strings.TrimSpace(parts[1])
		return code, state, nil
	}
	if strings.Contains(v, "code=") {
		u, parseErr := url.Parse(v)
		if parseErr == nil {
			q := u.Query()
			code = strings.TrimSpace(q.Get("code"))
			state = strings.TrimSpace(q.Get("state"))
			return code, state, nil
		}
		q, parseErr := url.ParseQuery(v)
		if parseErr == nil {
			code = strings.TrimSpace(q.Get("code"))
			state = strings.TrimSpace(q.Get("state"))
			return code, state, nil
		}
	}

	code = v
	return code, "", nil
}

func StartCodexOAuth(c *gin.Context) {
	startCodexOAuthWithChannelID(c, 0)
}

func StartCodexOAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}
	startCodexOAuthWithChannelID(c, channelID)
}

func startCodexOAuthWithChannelID(c *gin.Context, channelID int) {
	if channelID > 0 {
		ch, err := model.GetChannelById(channelID, false)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if ch == nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
			return
		}
		if ch.Type != constant.ChannelTypeCodex {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Codex"})
			return
		}
	}

	flow, err := service.CreateCodexOAuthAuthorizationFlow()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	session := sessions.Default(c)
	session.Set(codexOAuthSessionKey(channelID, "state"), flow.State)
	session.Set(codexOAuthSessionKey(channelID, "verifier"), flow.Verifier)
	session.Set(codexOAuthSessionKey(channelID, "created_at"), time.Now().Unix())
	_ = session.Save()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"authorize_url": flow.AuthorizeURL,
		},
	})
}

func CompleteCodexOAuth(c *gin.Context) {
	completeCodexOAuthWithChannelID(c, 0)
}

func CompleteCodexOAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}
	completeCodexOAuthWithChannelID(c, channelID)
}

func ImportCodexOAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	req := codexOAuthImportRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	appendMode := req.Append || c.Query("append") == "1" || strings.EqualFold(c.Query("append"), "true")
	keys, err := parseCodexOAuthImportInput(req.Input)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := importCodexOAuthKeysToChannel(channelID, keys, appendMode); err != nil {
		common.ApiError(c, err)
		return
	}

	model.InitChannelCache()
	service.ResetProxyClientCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": func() string {
			if appendMode {
				return "appended"
			}
			return "saved"
		}(),
		"data": gin.H{
			"channel_id": channelID,
			"count":      len(keys),
			"append":     appendMode,
		},
	})
}

func completeCodexOAuthWithChannelID(c *gin.Context, channelID int) {
	req := codexOAuthCompleteRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	code, state, err := parseCodexAuthorizationInput(req.Input)
	if err != nil {
		common.SysError("failed to parse codex authorization input: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "解析授权信息失败，请检查输入格式"})
		return
	}
	if strings.TrimSpace(code) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "missing authorization code"})
		return
	}
	if strings.TrimSpace(state) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "missing state in input"})
		return
	}

	channelProxy := ""
	if channelID > 0 {
		ch, err := model.GetChannelById(channelID, false)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if ch == nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
			return
		}
		if ch.Type != constant.ChannelTypeCodex {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Codex"})
			return
		}
		channelProxy = ch.GetSetting().Proxy
	}

	session := sessions.Default(c)
	expectedState, _ := session.Get(codexOAuthSessionKey(channelID, "state")).(string)
	verifier, _ := session.Get(codexOAuthSessionKey(channelID, "verifier")).(string)
	if strings.TrimSpace(expectedState) == "" || strings.TrimSpace(verifier) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "oauth flow not started or session expired"})
		return
	}
	if state != expectedState {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "state mismatch"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	tokenRes, err := service.ExchangeCodexAuthorizationCodeWithProxy(ctx, code, verifier, channelProxy)
	if err != nil {
		common.SysError("failed to exchange codex authorization code: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "授权码交换失败，请重试"})
		return
	}

	accountID, ok := service.ExtractCodexAccountIDFromJWT(tokenRes.AccessToken)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "failed to extract account_id from access_token"})
		return
	}
	email, _ := service.ExtractEmailFromJWT(tokenRes.AccessToken)

	key := codex.OAuthKey{
		AccessToken:  tokenRes.AccessToken,
		RefreshToken: tokenRes.RefreshToken,
		AccountID:    accountID,
		LastRefresh:  time.Now().Format(time.RFC3339),
		Expired:      tokenRes.ExpiresAt.Format(time.RFC3339),
		Email:        email,
		Type:         "codex",
	}
	encoded, err := common.Marshal(key)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	session.Delete(codexOAuthSessionKey(channelID, "state"))
	session.Delete(codexOAuthSessionKey(channelID, "verifier"))
	session.Delete(codexOAuthSessionKey(channelID, "created_at"))
	_ = session.Save()

	if channelID > 0 {
		appendMode := req.Append || c.Query("append") == "1" || strings.EqualFold(c.Query("append"), "true")
		if appendMode {
			if err := appendCodexOAuthKeyToChannel(channelID, key); err != nil {
				common.ApiError(c, err)
				return
			}
		} else if err := model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("key", string(encoded)).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		model.InitChannelCache()
		service.ResetProxyClientCache()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": func() string {
				if appendMode {
					return "appended"
				}
				return "saved"
			}(),
			"data": gin.H{
				"channel_id":   channelID,
				"account_id":   accountID,
				"email":        email,
				"expires_at":   key.Expired,
				"last_refresh": key.LastRefresh,
				"append":       appendMode,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "generated",
		"data": gin.H{
			"key":          string(encoded),
			"account_id":   accountID,
			"email":        email,
			"expires_at":   key.Expired,
			"last_refresh": key.LastRefresh,
		},
	})
}

func appendCodexOAuthKeyToChannel(channelID int, oauthKey codex.OAuthKey) error {
	return importCodexOAuthKeysToChannel(channelID, []codex.OAuthKey{oauthKey}, true)
}

func parseCodexOAuthImportInput(input string) ([]codex.OAuthKey, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return nil, errors.New("empty oauth input")
	}

	if key, err := parseSingleCodexOAuthImportKey(raw); err == nil {
		return []codex.OAuthKey{key}, nil
	}

	var envelope codexOAuthImportEnvelope
	if err := common.Unmarshal([]byte(raw), &envelope); err == nil {
		if envelope.Type == "sub2api-data" || len(envelope.Accounts) > 0 {
			keys := make([]codex.OAuthKey, 0, len(envelope.Accounts))
			for _, account := range envelope.Accounts {
				key, err := normalizeCodexOAuthImportKey(account.Credentials)
				if err != nil {
					return nil, err
				}
				keys = append(keys, key)
			}
			if len(keys) == 0 {
				return nil, errors.New("sub2api file does not contain any oauth credentials")
			}
			return keys, nil
		}
	}

	lines := strings.Split(raw, "\n")
	if len(lines) > 1 {
		keys := make([]codex.OAuthKey, 0, len(lines))
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			key, err := parseSingleCodexOAuthImportKey(trimmed)
			if err != nil {
				return nil, err
			}
			keys = append(keys, key)
		}
		if len(keys) > 0 {
			return keys, nil
		}
	}

	return nil, errors.New("unsupported codex oauth import format")
}

func parseSingleCodexOAuthImportKey(raw string) (codex.OAuthKey, error) {
	var imported codexOAuthImportKey
	if err := common.Unmarshal([]byte(raw), &imported); err != nil {
		return codex.OAuthKey{}, err
	}
	return normalizeCodexOAuthImportKey(imported)
}

func normalizeCodexOAuthImportKey(imported codexOAuthImportKey) (codex.OAuthKey, error) {
	key := imported.OAuthKey
	key.AccessToken = strings.TrimSpace(key.AccessToken)
	key.RefreshToken = strings.TrimSpace(key.RefreshToken)
	key.AccountID = strings.TrimSpace(key.AccountID)
	key.Email = strings.TrimSpace(key.Email)
	key.Type = strings.TrimSpace(key.Type)
	key.LastRefresh = strings.TrimSpace(key.LastRefresh)
	key.Expired = strings.TrimSpace(key.Expired)

	if key.AccessToken == "" {
		return codex.OAuthKey{}, errors.New("oauth credential must include access_token")
	}
	if key.AccountID == "" {
		key.AccountID = strings.TrimSpace(imported.ChatGPTAccountID)
	}
	if key.AccountID == "" {
		if accountID, ok := service.ExtractCodexAccountIDFromJWT(key.AccessToken); ok {
			key.AccountID = accountID
		}
	}
	if key.AccountID == "" {
		return codex.OAuthKey{}, errors.New("oauth credential must include account_id")
	}
	if key.Email == "" {
		if email, ok := service.ExtractEmailFromJWT(key.AccessToken); ok {
			key.Email = email
		}
	}
	if key.Type == "" {
		key.Type = "codex"
	}
	return key, nil
}

func importCodexOAuthKeysToChannel(channelID int, oauthKeys []codex.OAuthKey, appendMode bool) error {
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return err
	}
	if ch == nil {
		return errors.New("channel not found")
	}
	if ch.Type != constant.ChannelTypeCodex {
		return errors.New("channel type is not Codex")
	}

	if len(oauthKeys) == 0 {
		return errors.New("empty oauth key list")
	}

	importedByAccountID := make(map[string]string, len(oauthKeys))
	importedOrder := make([]string, 0, len(oauthKeys))
	for _, oauthKey := range oauthKeys {
		normalizedKey, err := normalizeCodexOAuthImportKey(codexOAuthImportKey{OAuthKey: oauthKey})
		if err != nil {
			return err
		}
		encoded, err := common.Marshal(normalizedKey)
		if err != nil {
			return err
		}
		if _, exists := importedByAccountID[normalizedKey.AccountID]; !exists {
			importedOrder = append(importedOrder, normalizedKey.AccountID)
		}
		importedByAccountID[normalizedKey.AccountID] = string(encoded)
	}

	merged := make([]string, 0, len(importedOrder)+len(ch.GetKeys()))
	if appendMode {
		for _, rawKey := range ch.GetKeys() {
			trimmed := strings.TrimSpace(rawKey)
			if trimmed == "" {
				continue
			}
			existing, parseErr := codex.ParseOAuthKey(trimmed)
			if parseErr == nil && strings.TrimSpace(existing.AccountID) != "" {
				if replacement, ok := importedByAccountID[existing.AccountID]; ok {
					merged = append(merged, replacement)
					delete(importedByAccountID, existing.AccountID)
					continue
				}
			}
			merged = append(merged, trimmed)
		}
	}
	for _, accountID := range importedOrder {
		if encoded, ok := importedByAccountID[accountID]; ok {
			merged = append(merged, encoded)
		}
	}

	ch.Key = strings.Join(merged, "\n")
	if len(merged) > 1 {
		ch.ChannelInfo.IsMultiKey = true
		if ch.ChannelInfo.MultiKeyMode == "" {
			ch.ChannelInfo.MultiKeyMode = constant.MultiKeyModeRandom
		}
	} else {
		ch.ChannelInfo.IsMultiKey = false
	}
	ch.ChannelInfo.MultiKeySize = len(merged)
	return ch.Update()
}
