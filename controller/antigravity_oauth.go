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
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type antigravityOAuthCompleteRequest struct {
	Input string `json:"input"`
}

func clearAntigravityOAuthSession(session sessions.Session, channelID int) {
	session.Delete(antigravityOAuthSessionKey(channelID, "state"))
	session.Delete(antigravityOAuthSessionKey(channelID, "verifier"))
	session.Delete(antigravityOAuthSessionKey(channelID, "created_at"))
	_ = session.Save()
}

func antigravityOAuthSessionKey(channelID int, field string) string {
	return fmt.Sprintf("antigravity_oauth_%s_%d", field, channelID)
}

func parseAntigravityAuthorizationInput(input string) (code string, state string, err error) {
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

func StartAntigravityOAuth(c *gin.Context) {
	startAntigravityOAuthWithChannelID(c, 0)
}

func StartAntigravityOAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}
	startAntigravityOAuthWithChannelID(c, channelID)
}

func startAntigravityOAuthWithChannelID(c *gin.Context, channelID int) {
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
		if ch.Type != constant.ChannelTypeAntigravity {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Antigravity"})
			return
		}
	}

	flow, err := service.CreateAntigravityOAuthAuthorizationFlow()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	session := sessions.Default(c)
	session.Set(antigravityOAuthSessionKey(channelID, "state"), flow.State)
	session.Set(antigravityOAuthSessionKey(channelID, "verifier"), flow.Verifier)
	session.Set(antigravityOAuthSessionKey(channelID, "created_at"), time.Now().Unix())
	_ = session.Save()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"authorize_url": flow.AuthorizeURL,
		},
	})
}

func CompleteAntigravityOAuth(c *gin.Context) {
	completeAntigravityOAuthWithChannelID(c, 0)
}

func CompleteAntigravityOAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}
	completeAntigravityOAuthWithChannelID(c, channelID)
}

func completeAntigravityOAuthWithChannelID(c *gin.Context, channelID int) {
	req := antigravityOAuthCompleteRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	code, state, err := parseAntigravityAuthorizationInput(req.Input)
	if err != nil {
		common.SysError("failed to parse antigravity authorization input: " + err.Error())
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
		if ch.Type != constant.ChannelTypeAntigravity {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Antigravity"})
			return
		}
		channelProxy = ch.GetSetting().Proxy
	}

	session := sessions.Default(c)
	expectedState, _ := session.Get(antigravityOAuthSessionKey(channelID, "state")).(string)
	verifier, _ := session.Get(antigravityOAuthSessionKey(channelID, "verifier")).(string)
	if strings.TrimSpace(expectedState) == "" || strings.TrimSpace(verifier) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "oauth flow not started or session expired"})
		return
	}
	if state != expectedState {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "state mismatch"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	tokenRes, err := service.ExchangeAntigravityAuthorizationCodeWithProxy(ctx, code, verifier, channelProxy)
	if err != nil {
		common.SysError("failed to exchange antigravity authorization code: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "授权码交换失败，请重试"})
		return
	}

	var email string
	userInfo, userInfoErr := service.FetchAntigravityUserInfoWithProxy(ctx, tokenRes.AccessToken, channelProxy)
	if userInfoErr == nil && userInfo != nil {
		email = strings.TrimSpace(userInfo.Email)
	}

	projectID, projectErr := service.FetchAntigravityProjectIDWithProxy(ctx, tokenRes.AccessToken, channelProxy)
	if projectErr != nil || strings.TrimSpace(projectID) == "" {
		if projectErr != nil {
			common.SysError("failed to fetch antigravity project_id: " + projectErr.Error())
		} else {
			common.SysError("failed to fetch antigravity project_id: empty project_id")
		}
		clearAntigravityOAuthSession(session, channelID)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "failed to fetch Antigravity project_id after authorization, please restart OAuth",
		})
		return
	}

	key := service.AntigravityOAuthKey{
		AccessToken:  tokenRes.AccessToken,
		RefreshToken: tokenRes.RefreshToken,
		TokenType:    tokenRes.TokenType,
		ProjectID:    projectID,
		LastRefresh:  time.Now().Format(time.RFC3339),
		Expired:      tokenRes.ExpiresAt.Format(time.RFC3339),
		Email:        email,
		Type:         "antigravity",
	}
	encoded, err := common.Marshal(key)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	clearAntigravityOAuthSession(session, channelID)

	if channelID > 0 {
		if err := model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("key", string(encoded)).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		model.InitChannelCache()
		service.ResetProxyClientCache()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "saved",
			"data": gin.H{
				"channel_id":   channelID,
				"email":        email,
				"project_id":   projectID,
				"expires_at":   key.Expired,
				"last_refresh": key.LastRefresh,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "generated",
		"data": gin.H{
			"key":          string(encoded),
			"email":        email,
			"project_id":   projectID,
			"expires_at":   key.Expired,
			"last_refresh": key.LastRefresh,
		},
	})
}
