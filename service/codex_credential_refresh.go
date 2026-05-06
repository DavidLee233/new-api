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

type CodexCredentialRefreshOptions struct {
	ResetCaches bool
}

type CodexOAuthKey struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`

	AccountID   string `json:"account_id,omitempty"`
	LastRefresh string `json:"last_refresh,omitempty"`
	Email       string `json:"email,omitempty"`
	Type        string `json:"type,omitempty"`
	Expired     string `json:"expired,omitempty"`
}

func parseCodexOAuthKey(raw string) (*CodexOAuthKey, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("codex channel: empty oauth key")
	}
	var key CodexOAuthKey
	if err := common.Unmarshal([]byte(raw), &key); err != nil {
		return nil, errors.New("codex channel: invalid oauth key json")
	}
	return &key, nil
}

func parseCodexOAuthKeys(raw string) ([]*CodexOAuthKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("codex channel: empty oauth key")
	}
	channel := &model.Channel{Key: raw}
	rawKeys := channel.GetKeys()
	keys := make([]*CodexOAuthKey, 0, len(rawKeys))
	for _, item := range rawKeys {
		key, err := parseCodexOAuthKey(strings.TrimSpace(item))
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, errors.New("codex channel: empty oauth key")
	}
	return keys, nil
}

func marshalCodexOAuthKeys(keys []*CodexOAuthKey) (string, error) {
	if len(keys) == 0 {
		return "", errors.New("codex channel: empty oauth key")
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
		return "", errors.New("codex channel: empty oauth key")
	}
	return strings.Join(parts, "\n"), nil
}

func RefreshCodexChannelCredential(ctx context.Context, channelID int, opts CodexCredentialRefreshOptions) (*CodexOAuthKey, *model.Channel, error) {
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if ch == nil {
		return nil, nil, fmt.Errorf("channel not found")
	}
	if ch.Type != constant.ChannelTypeCodex {
		return nil, nil, fmt.Errorf("channel type is not Codex")
	}

	oauthKeys, err := parseCodexOAuthKeys(strings.TrimSpace(ch.Key))
	if err != nil {
		return nil, nil, err
	}
	var refreshedFirst *CodexOAuthKey
	for _, oauthKey := range oauthKeys {
		if strings.TrimSpace(oauthKey.RefreshToken) == "" {
			return nil, nil, fmt.Errorf("codex channel: refresh_token is required to refresh credential")
		}

		refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		res, err := RefreshCodexOAuthTokenWithProxy(refreshCtx, oauthKey.RefreshToken, ch.GetSetting().Proxy)
		cancel()
		if err != nil {
			return nil, nil, err
		}

		oauthKey.AccessToken = res.AccessToken
		oauthKey.RefreshToken = res.RefreshToken
		oauthKey.LastRefresh = time.Now().Format(time.RFC3339)
		oauthKey.Expired = res.ExpiresAt.Format(time.RFC3339)
		if strings.TrimSpace(oauthKey.Type) == "" {
			oauthKey.Type = "codex"
		}

		if strings.TrimSpace(oauthKey.AccountID) == "" {
			if accountID, ok := ExtractCodexAccountIDFromJWT(oauthKey.AccessToken); ok {
				oauthKey.AccountID = accountID
			}
		}
		if strings.TrimSpace(oauthKey.Email) == "" {
			if email, ok := ExtractEmailFromJWT(oauthKey.AccessToken); ok {
				oauthKey.Email = email
			}
		}
		if refreshedFirst == nil {
			refreshedFirst = oauthKey
		}
	}

	encoded, err := marshalCodexOAuthKeys(oauthKeys)
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

	return refreshedFirst, ch, nil
}
