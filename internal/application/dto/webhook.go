package dto

import "time"

type CreateWebhookRequest struct {
	URL       string `json:"url" binding:"required,url"`
	EventType string `json:"event_type" binding:"required"`
	Secret    string `json:"secret"`
}

type WebhookResponse struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	EventType string    `json:"event_type"`
	Secret    string    `json:"secret"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}
