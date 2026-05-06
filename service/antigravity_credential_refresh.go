package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type AntigravityCredentialRefreshOptions struct {
	ResetCaches bool
}

type AntigravityOAuthKey struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`

	ProjectID   string `json:"project_id,omitempty"`
	LastRefresh string `json:"last_refresh,omitempty"`
	Email       string `json:"email,omitempty"`
	Type        string `json:"type,omitempty"`
	Expired     string `json:"expired,omitempty"`
}

func parseAntigravityOAuthKey(raw string) (*AntigravityOAuthKey, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("antigravity channel: empty oauth key")
	}
	var key AntigravityOAuthKey
	if err := common.Unmarshal([]byte(raw), &key); err != nil {
		return nil, errors.New("antigravity channel: invalid oauth key json")
	}
	return &key, nil
}

func RefreshAntigravityChannelCredential(ctx context.Context, channelID int, opts AntigravityCredentialRefreshOptions) (*AntigravityOAuthKey, *model.Channel, error) {
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if ch == nil {
		return nil, nil, fmt.Errorf("channel not found")
	}
	if ch.Type != constant.ChannelTypeAntigravity {
		return nil, nil, fmt.Errorf("channel type is not Antigravity")
	}

	oauthKey, err := parseAntigravityOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return nil, nil, fmt.Errorf("antigravity channel: refresh_token is required to refresh credential")
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	res, err := RefreshAntigravityOAuthTokenWithProxy(refreshCtx, oauthKey.RefreshToken, ch.GetSetting().Proxy)
	if err != nil {
		return nil, nil, err
	}

	oauthKey.AccessToken = res.AccessToken
	if strings.TrimSpace(res.RefreshToken) != "" {
		oauthKey.RefreshToken = res.RefreshToken
	}
	if strings.TrimSpace(res.TokenType) != "" {
		oauthKey.TokenType = res.TokenType
	}
	oauthKey.LastRefresh = time.Now().Format(time.RFC3339)
	oauthKey.Expired = res.ExpiresAt.Format(time.RFC3339)
	if strings.TrimSpace(oauthKey.Type) == "" {
		oauthKey.Type = "antigravity"
	}

	if userInfo, userErr := FetchAntigravityUserInfoWithProxy(refreshCtx, oauthKey.AccessToken, ch.GetSetting().Proxy); userErr == nil && userInfo != nil && strings.TrimSpace(userInfo.Email) != "" {
		oauthKey.Email = strings.TrimSpace(userInfo.Email)
	}
	projectID, projectErr := FetchAntigravityProjectIDWithProxy(refreshCtx, oauthKey.AccessToken, ch.GetSetting().Proxy)
	if projectErr == nil && strings.TrimSpace(projectID) != "" {
		oauthKey.ProjectID = strings.TrimSpace(projectID)
	}
	if strings.TrimSpace(oauthKey.ProjectID) == "" {
		if projectErr != nil {
			return nil, nil, fmt.Errorf("antigravity channel: failed to refresh project_id: %w", projectErr)
		}
		return nil, nil, fmt.Errorf("antigravity channel: project_id is missing after refresh")
	}

	encoded, err := common.Marshal(oauthKey)
	if err != nil {
		return nil, nil, err
	}

	if err := model.DB.Model(&model.Channel{}).Where("id = ?", ch.Id).Update("key", string(encoded)).Error; err != nil {
		return nil, nil, err
	}

	if opts.ResetCaches {
		model.InitChannelCache()
		ResetProxyClientCache()
	}

	return oauthKey, ch, nil
}
