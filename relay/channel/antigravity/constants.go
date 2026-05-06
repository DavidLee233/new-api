package antigravity

import "time"

const (
	ChannelName = "antigravity"

	antigravityBaseURLProd         = "https://cloudcode-pa.googleapis.com"
	antigravityBaseURLDaily        = "https://daily-cloudcode-pa.googleapis.com"
	antigravitySandboxBaseURLDaily = "https://daily-cloudcode-pa.sandbox.googleapis.com"

	googleRPCStatusResourceExhausted      = "RESOURCE_EXHAUSTED"
	googleRPCStatusUnavailable            = "UNAVAILABLE"
	googleRPCTypeRetryInfo                = "type.googleapis.com/google.rpc.RetryInfo"
	googleRPCTypeErrorInfo                = "type.googleapis.com/google.rpc.ErrorInfo"
	googleRPCReasonModelCapacityExhausted = "MODEL_CAPACITY_EXHAUSTED"
	googleRPCReasonRateLimitExceeded      = "RATE_LIMIT_EXCEEDED"
)

const (
	antigravityDefaultRateLimitDuration = 30 * time.Second
	antigravityRateLimitThreshold       = 7 * time.Second
	antigravitySmartRetryMinWait        = 500 * time.Millisecond
	antigravitySmartRetryMaxAttempts    = 3

	antigravityModelCapacityRetryMaxAttempts = 5
	antigravityModelCapacityRetryWait        = 1 * time.Second
)
