package hcaptcha

import "time"

// Response is an hCaptcha response
type Response struct {
	Success     bool        `json:"success"`
	ChallengeTS time.Time   `json:"challenge_ts"`
	Hostname    string      `json:"hostname"`
	Credit      bool        `json:"credit,omitempty"`
	ErrorCodes  []ErrorCode `json:"error-codes"`
}
