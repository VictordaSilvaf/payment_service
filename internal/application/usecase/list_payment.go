package usecase

import (
	"context"
	"math"
	"strconv"

	"payment_service/internal/application/dto"
	"payment_service/internal/domain/payment"
)

type ListPayment struct {
	repo payment.Repository
}

func NewListPayment(repo payment.Repository) *ListPayment {
	return &ListPayment{repo: repo}
}

func (uc *ListPayment) Execute(ctx context.Context, page, limit, sort, order, status string) (*dto.PaginatedResponse, error) {
	pageResult, err := uc.repo.FindAll(ctx, page, limit, sort, order, status)
	if err != nil {
		return nil, err
	}

	if page == "" {
		page = "1"
	}
	if limit == "" {
		limit = "10"
	}

	limitInt := parsePositiveInt(limit, 10)
	totalPages := 0
	if limitInt > 0 && pageResult.Total > 0 {
		totalPages = int(math.Ceil(float64(pageResult.Total) / float64(limitInt)))
	}

	return &dto.PaginatedResponse{
		Data:       toPaymentResponses(pageResult.Items),
		Page:       page,
		Limit:      limit,
		Total:      pageResult.Total,
		TotalPages: totalPages,
	}, nil
}

func toPaymentResponses(payments []*payment.Payment) []*dto.PaymentResponse {
	responses := make([]*dto.PaymentResponse, len(payments))
	for i, p := range payments {
		responses[i] = toPaymentResponse(p)
	}
	return responses
}

func parsePositiveInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
