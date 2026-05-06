package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type codexUsageAccountPayload struct {
	Index          int    `json:"index"`
	AccountID      string `json:"account_id,omitempty"`
	Email          string `json:"email,omitempty"`
	Success        bool   `json:"success"`
	Message        string `json:"message,omitempty"`
	UpstreamStatus int    `json:"upstream_status"`
	Data           any    `json:"data,omitempty"`
}

func parseCodexUsageOAuthKeys(raw string) ([]*codex.OAuthKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("codex channel: empty oauth key")
	}

	channel := &model.Channel{Key: raw}
	rawKeys := channel.GetKeys()
	keys := make([]*codex.OAuthKey, 0, len(rawKeys))
	for _, item := range rawKeys {
		key, err := codex.ParseOAuthKey(strings.TrimSpace(item))
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("codex channel: empty oauth key")
	}
	return keys, nil
}

func marshalCodexUsageOAuthKeys(keys []*codex.OAuthKey) (string, error) {
	if len(keys) == 0 {
		return "", fmt.Errorf("codex channel: empty oauth key")
	}

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == nil {
			continue
		}
		data, err := common.Marshal(key)
		if err != nil {
			return "", err
		}
		parts = append(parts, string(data))
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("codex channel: empty oauth key")
	}
	return strings.Join(parts, "\n"), nil
}

func decodeCodexUsageBody(body []byte) any {
	var payload any
	if err := common.Unmarshal(body, &payload); err != nil {
		return string(body)
	}
	return payload
}

func fetchCodexUsageForAccount(
	ctx context.Context,
	client *http.Client,
	ch *model.Channel,
	oauthKey *codex.OAuthKey,
) (statusCode int, body []byte, refreshed bool, err error) {
	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	accountID := strings.TrimSpace(oauthKey.AccountID)
	if accessToken == "" {
		return 0, nil, false, fmt.Errorf("codex channel: access_token is required")
	}
	if accountID == "" {
		return 0, nil, false, fmt.Errorf("codex channel: account_id is required")
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	statusCode, body, err = service.FetchCodexWhamUsage(reqCtx, client, ch.GetBaseURL(), accessToken, accountID)
	if err != nil {
		return statusCode, body, false, err
	}

	if (statusCode != http.StatusUnauthorized && statusCode != http.StatusForbidden) || strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return statusCode, body, false, nil
	}

	refreshCtx, refreshCancel := context.WithTimeout(ctx, 10*time.Second)
	defer refreshCancel()

	res, refreshErr := service.RefreshCodexOAuthTokenWithProxy(refreshCtx, oauthKey.RefreshToken, ch.GetSetting().Proxy)
	if refreshErr != nil {
		return statusCode, body, false, nil
	}

	oauthKey.AccessToken = res.AccessToken
	oauthKey.RefreshToken = res.RefreshToken
	oauthKey.LastRefresh = time.Now().Format(time.RFC3339)
	oauthKey.Expired = res.ExpiresAt.Format(time.RFC3339)
	if strings.TrimSpace(oauthKey.Type) == "" {
		oauthKey.Type = "codex"
	}

	reqCtx2, cancel2 := context.WithTimeout(ctx, 15*time.Second)
	defer cancel2()

	statusCode, body, err = service.FetchCodexWhamUsage(reqCtx2, client, ch.GetBaseURL(), oauthKey.AccessToken, accountID)
	return statusCode, body, true, err
}

func GetCodexChannelUsage(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ch, err := model.GetChannelById(channelId, true)
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

	oauthKeys, err := parseCodexUsageOAuthKeys(ch.Key)
	if err != nil {
		common.SysError("failed to parse oauth key: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "解析凭据失败，请检查渠道配置"})
		return
	}

	client, err := service.NewProxyHttpClient(ch.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	accounts := make([]codexUsageAccountPayload, 0, len(oauthKeys))
	var anySuccess bool
	var refreshedAny bool

	for idx, oauthKey := range oauthKeys {
		account := codexUsageAccountPayload{
			Index:     idx,
			AccountID: strings.TrimSpace(oauthKey.AccountID),
			Email:     strings.TrimSpace(oauthKey.Email),
		}

		statusCode, body, refreshed, fetchErr := fetchCodexUsageForAccount(c.Request.Context(), client, ch, oauthKey)
		if refreshed {
			refreshedAny = true
		}
		account.UpstreamStatus = statusCode

		if fetchErr != nil {
			common.SysError("failed to fetch codex usage: " + fetchErr.Error())
			account.Success = false
			account.Message = "获取用量信息失败，请稍后重试"
			accounts = append(accounts, account)
			continue
		}

		account.Success = statusCode >= 200 && statusCode < 300
		account.Data = decodeCodexUsageBody(body)
		if !account.Success {
			account.Message = fmt.Sprintf("upstream status: %d", statusCode)
		} else {
			anySuccess = true
		}

		accounts = append(accounts, account)
	}

	if refreshedAny {
		if encoded, marshalErr := marshalCodexUsageOAuthKeys(oauthKeys); marshalErr == nil {
			_ = model.DB.Model(&model.Channel{}).Where("id = ?", ch.Id).Update("key", encoded).Error
			model.InitChannelCache()
			service.ResetProxyClientCache()
		}
	}

	var firstData any
	if len(accounts) > 0 {
		firstData = accounts[0].Data
	}

	failedCount := len(accounts)
	if anySuccess {
		failedCount = 0
		for _, account := range accounts {
			if !account.Success {
				failedCount++
			}
		}
	}

	message := ""
	if len(accounts) == 0 {
		message = "channel has no oauth accounts"
	} else if !anySuccess {
		message = "all accounts failed"
	} else if failedCount > 0 {
		message = fmt.Sprintf("%d accounts failed", failedCount)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         anySuccess,
		"message":         message,
		"upstream_status": 0,
		"data":            firstData,
		"accounts":        accounts,
		"summary": gin.H{
			"total_accounts":   len(accounts),
			"success_accounts": len(accounts) - failedCount,
			"failed_accounts":  failedCount,
			"is_multi_account": len(accounts) > 1,
		},
	})
}
