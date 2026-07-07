package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"payment_service/internal/application/idempotency"
	"payment_service/internal/application/usecase"
	"payment_service/internal/infrastructure/http/handler"
	"payment_service/internal/infrastructure/persistence/memory"
	"payment_service/internal/testutil"
)

func TestNewRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := memory.NewPaymentRepository()
	idempotencyService := idempotency.NewService(testutil.NewMemoryIdempotencyRepo())

	router := NewRouter(RouterConfig{
		HealthHandler: handler.NewHealthHandler(),
		PaymentHandler: handler.NewPaymentHandler(
			usecase.NewCreatePayment(repo, memory.NewOutboxRepository(), nil),
			usecase.NewGetPayment(repo),
			usecase.NewListPayment(repo),
			nil,
			nil,
			idempotencyService,
		),
	})

	t.Run("ping", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})
}
