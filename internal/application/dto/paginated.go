package dto

type PaginatedResponse struct {
	Data       []*PaymentResponse `json:"data"`
	Page       string             `json:"page"`
	Limit      string             `json:"limit"`
	Total      int                `json:"total"`
	TotalPages int                `json:"total_pages"`
}
