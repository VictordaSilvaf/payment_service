package webhook

import "errors"

var (
	// ErrInvalidURL indica URL ausente ou com esquema diferente de http/https.
	ErrInvalidURL = errors.New("webhook url is required and must be http or https")
	// ErrInvalidEventType indica event_type vazio.
	ErrInvalidEventType = errors.New("webhook event_type is required")
)
