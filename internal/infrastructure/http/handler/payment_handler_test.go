package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"payment_service/internal/application/dto"
	"payment_service/internal/application/idempotency"
	"payment_service/internal/application/usecase"
	"payment_service/internal/domain/payment"
	"payment_service/internal/infrastructure/http/middleware"
	"payment_service/internal/infrastructure/persistence/memory"
	"payment_service/internal/testutil"
)

func setupPaymentRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := memory.NewPaymentRepository()
	idempotencyService := idempotency.NewService(testutil.NewMemoryIdempotencyRepo())
	paymentHandler := NewPaymentHandler(
		usecase.NewCreatePayment(repo, memory.NewOutboxRepository(), nil),
		usecase.NewGetPayment(repo),
		usecase.NewListPayment(repo),
		idempotencyService,
	)

	router := gin.New()
	router.GET("/ping", NewHealthHandler().Ping)
	router.POST("/api/v1/payments", middleware.Idempotency(), paymentHandler.Create)
	router.GET("/api/v1/payments/:id", paymentHandler.GetByID)
	router.GET("/api/v1/payments", paymentHandler.List)

	return router
}

func TestHealthHandlerPing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	NewHealthHandler().Ping(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestPaymentHandlerCreate(t *testing.T) {
	router := setupPaymentRouter(t)

	body, _ := json.Marshal(map[string]any{"amount": 1500, "currency": "BRL"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "create-key-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "create-key-1")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusCreated || w.Body.String() != w2.Body.String() {
		t.Fatalf("expected cached identical response, got %d body=%s", w2.Code, w2.Body.String())
	}
}

func TestPaymentHandlerCreateValidation(t *testing.T) {
	router := setupPaymentRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewReader([]byte(`{"amount":0,"currency":"BRL"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "bad-amount")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPaymentHandlerGetByID(t *testing.T) {
	router := setupPaymentRouter(t)

	createBody, _ := json.Marshal(map[string]any{"amount": 500, "currency": "BRL"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Idempotency-Key", "get-setup")
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)

	var created map[string]any
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+created["id"].(string), nil)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getW.Code, getW.Body.String())
	}
}

func TestPaymentHandlerGetByIDNotFound(t *testing.T) {
	router := setupPaymentRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/missing-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPaymentHandlerList(t *testing.T) {
	router := setupPaymentRouter(t)

	for i := range 2 {
		body, _ := json.Marshal(map[string]any{"amount": 100 * (i + 1), "currency": "BRL"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", fmt.Sprintf("list-key-%d", i))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments?page=1&limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPaymentHandlerCreateWithoutIdempotencyKey(t *testing.T) {
	router := setupPaymentRouter(t)

	body, _ := json.Marshal(map[string]any{"amount": 100, "currency": "BRL"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPaymentHandlerCreateKeyConflict(t *testing.T) {
	router := setupPaymentRouter(t)

	body1, _ := json.Marshal(map[string]any{"amount": 100, "currency": "BRL"})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", "conflict-key")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	body2, _ := json.Marshal(map[string]any{"amount": 999, "currency": "BRL"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "conflict-key")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", w2.Code, w2.Body.String())
	}
}

func TestHashRequest(t *testing.T) {
	req := dto.CreatePaymentRequest{Amount: 100, Currency: "BRL"}
	hash1, err := hashRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	hash2, err := hashRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 != hash2 {
		t.Fatal("expected same hash for same request")
	}
}

func TestPaymentHandlerCreateInvalidJSON(t *testing.T) {
	router := setupPaymentRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewReader([]byte(`{invalid`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "bad-json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPaymentHandlerCreateInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repoErr := fmt.Errorf("db unavailable")
	idempotencyService := idempotency.NewService(testutil.NewMemoryIdempotencyRepo())
	paymentHandler := NewPaymentHandler(
		usecase.NewCreatePayment(&testutil.ErrorPaymentRepository{SaveErr: repoErr}, memory.NewOutboxRepository(), nil),
		usecase.NewGetPayment(memory.NewPaymentRepository()),
		usecase.NewListPayment(memory.NewPaymentRepository()),
		idempotencyService,
	)

	router := gin.New()
	router.POST("/api/v1/payments", middleware.Idempotency(), paymentHandler.Create)

	body, _ := json.Marshal(map[string]any{"amount": 100, "currency": "BRL"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "internal-error")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPaymentHandlerListInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repoErr := fmt.Errorf("list unavailable")
	paymentHandler := NewPaymentHandler(
		usecase.NewCreatePayment(memory.NewPaymentRepository(), memory.NewOutboxRepository(), nil),
		usecase.NewGetPayment(memory.NewPaymentRepository()),
		usecase.NewListPayment(&testutil.ErrorPaymentRepository{SaveErr: repoErr}),
		idempotency.NewService(testutil.NewMemoryIdempotencyRepo()),
	)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/payments", nil)
	paymentHandler.List(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestPaymentHandlerHandleError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewPaymentHandler(nil, nil, nil, nil)

	cases := []struct {
		name       string
		err        error
		statusCode int
	}{
		{"not found", payment.ErrNotFound, http.StatusNotFound},
		{"invalid amount", payment.ErrInvalidAmount, http.StatusBadRequest},
		{"invalid key", idempotency.ErrInvalidKey, http.StatusBadRequest},
		{"processing", idempotency.ErrAlreadyProcessing, http.StatusConflict},
		{"key exists", idempotency.ErrKeyAlreadyExists, http.StatusConflict},
		{"internal", fmt.Errorf("unknown"), http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			h.handleError(c, tc.err)
			if w.Code != tc.statusCode {
				t.Fatalf("expected %d, got %d", tc.statusCode, w.Code)
			}
		})
	}
}
