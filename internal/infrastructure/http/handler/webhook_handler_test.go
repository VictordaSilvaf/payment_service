package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	appwebhook "payment_service/internal/application/webhook"
	"payment_service/internal/infrastructure/persistence/memory"
)

func setupWebhookRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := memory.NewWebhookSubscriptionRepository()
	h := NewWebhookHandler(
		appwebhook.NewCreateSubscription(repo),
		appwebhook.NewListSubscriptions(repo),
	)

	router := gin.New()
	router.POST("/api/v1/webhooks", h.Create)
	router.GET("/api/v1/webhooks", h.List)
	return router
}

func TestWebhookHandlerCreate(t *testing.T) {
	router := setupWebhookRouter(t)

	body, _ := json.Marshal(map[string]any{
		"url":        "https://merchant.test/hook",
		"event_type": "payment.completed",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestWebhookHandlerCreateInvalidBody(t *testing.T) {
	router := setupWebhookRouter(t)

	// event_type ausente → binding falha.
	body, _ := json.Marshal(map[string]any{"url": "https://merchant.test/hook"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWebhookHandlerList(t *testing.T) {
	router := setupWebhookRouter(t)

	create, _ := json.Marshal(map[string]any{
		"url":        "https://merchant.test/hook",
		"event_type": "payment.completed",
	})
	cReq := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewReader(create))
	cReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(httptest.NewRecorder(), cReq)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(res))
	}
}
