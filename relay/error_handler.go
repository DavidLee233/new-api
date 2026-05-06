package relay

import (
	"net/http"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/antigravity"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func HandleUpstreamErrorResponse(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, showBodyWhenFail bool) *types.NewAPIError {
	if info != nil && info.ChannelType == constant.ChannelTypeAntigravity {
		return antigravity.BuildRelayError(c, info, resp, showBodyWhenFail)
	}
	if c != nil && c.Request != nil {
		return service.RelayErrorHandler(c.Request.Context(), resp, showBodyWhenFail)
	}
	return service.RelayErrorHandler(nil, resp, showBodyWhenFail)
}
