package idempotency

import (
	"context"
	"strings"
)

const maxKeyLength = 255

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Execute(
	ctx context.Context,
	key string,
	requestHash string,
	fn func(ctx context.Context) (CachedResponse, error),
) (CachedResponse, error) {
	if err := validateKey(key); err != nil {
		return CachedResponse{}, err
	}

	if cached, found, err := s.repository.Find(ctx, key); err != nil {
		return CachedResponse{}, err
	} else if found {
		return s.matchOrConflict(cached, requestHash)
	}

	acquired, err := s.repository.Lock(ctx, key)
	if err != nil {
		return CachedResponse{}, err
	}
	if !acquired {
		if cached, found, findErr := s.repository.Find(ctx, key); findErr != nil {
			return CachedResponse{}, findErr
		} else if found {
			return s.matchOrConflict(cached, requestHash)
		}
		return CachedResponse{}, ErrAlreadyProcessing
	}

	defer s.repository.Unlock(ctx, key)

	response, err := fn(ctx)
	if err != nil {
		return CachedResponse{}, err
	}

	response.RequestHash = requestHash
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		if err := s.repository.Save(ctx, key, response); err != nil {
			return CachedResponse{}, err
		}
	}

	return response, nil
}

func (s *Service) matchOrConflict(cached CachedResponse, requestHash string) (CachedResponse, error) {
	if cached.RequestHash != "" && cached.RequestHash != requestHash {
		return CachedResponse{}, ErrKeyAlreadyExists
	}
	return cached, nil
}

func validateKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > maxKeyLength {
		return ErrInvalidKey
	}
	return nil
}
