package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"payment_service/internal/application/dto"
	appwebhook "payment_service/internal/application/webhook"
	"payment_service/internal/domain/webhook"
)

type WebhookHandler struct {
	createSubscription *appwebhook.CreateSubscription
	listSubscriptions  *appwebhook.ListSubscriptions
}

func NewWebhookHandler(
	createSubscription *appwebhook.CreateSubscription,
	listSubscriptions *appwebhook.ListSubscriptions,
) *WebhookHandler {
	return &WebhookHandler{
		createSubscription: createSubscription,
		listSubscriptions:  listSubscriptions,
	}
}

func (h *WebhookHandler) Create(c *gin.Context) {
	var req dto.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := h.createSubscription.Execute(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, webhook.ErrInvalidURL), errors.Is(err, webhook.ErrInvalidEventType):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusCreated, res)
}

func (h *WebhookHandler) List(c *gin.Context) {
	res, err := h.listSubscriptions.Execute(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, res)
}
