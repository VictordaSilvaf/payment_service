package idempotency

import "encoding/json"

type CachedResponse struct {
	StatusCode  int             `json:"status_code"`
	Body        json.RawMessage `json:"body"`
	RequestHash string          `json:"request_hash,omitempty"`
}
