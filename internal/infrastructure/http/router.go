package http

import (
	"github.com/gin-gonic/gin"

	"payment_service/internal/infrastructure/http/handler"
	"payment_service/internal/infrastructure/http/middleware"
)

type RouterConfig struct {
	HealthHandler  *handler.HealthHandler
	PaymentHandler *handler.PaymentHandler
	WebhookHandler *handler.WebhookHandler
}

func NewRouter(cfg RouterConfig) *gin.Engine {
	router := gin.Default()

	router.GET("/ping", cfg.HealthHandler.Ping)

	v1 := router.Group("/api/v1")
	{
		v1.POST("/payments", middleware.Idempotency(), cfg.PaymentHandler.Create)
		v1.GET("/payments/:id", cfg.PaymentHandler.GetByID)
		v1.GET("/payments", cfg.PaymentHandler.List)

		// Registrado apenas quando o handler está configurado (nil em alguns testes).
		if cfg.WebhookHandler != nil {
			v1.POST("/webhooks", cfg.WebhookHandler.Create)
			v1.GET("/webhooks", cfg.WebhookHandler.List)
		}
	}

	return router
}
