package idempotency

import "errors"

var (
	ErrAlreadyProcessing = errors.New("request already processing")
	ErrInvalidKey        = errors.New("invalid idempotency key")
	ErrKeyAlreadyExists  = errors.New("idempotency key already exists")
)
