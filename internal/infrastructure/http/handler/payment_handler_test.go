package handler

import (
	"bytes"
	"context"
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
	"payment_service/internal/infrastructure/psp"
	"payment_service/internal/testutil"
)

func setupPaymentRouter(t *testing.T) *gin.Engine {
	router, _ := setupPaymentRouterWithRepo(t)
	return router
}

func setupPaymentRouterWithRepo(t *testing.T) (*gin.Engine, *memory.PaymentRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := memory.NewPaymentRepository()
	outboxRepo := memory.NewOutboxRepository()
	gateway := psp.NewMockGateway(0)
	idempotencyService := idempotency.NewService(testutil.NewMemoryIdempotencyRepo())
	paymentHandler := NewPaymentHandler(
		usecase.NewCreatePayment(repo, outboxRepo, nil),
		usecase.NewGetPayment(repo),
		usecase.NewListPayment(repo),
		usecase.NewCapturePayment(repo, gateway, outboxRepo, nil),
		usecase.NewRefundPayment(repo, gateway, outboxRepo, nil),
		idempotencyService,
	)

	router := gin.New()
	router.GET("/ping", NewHealthHandler().Ping)
	router.POST("/api/v1/payments", middleware.Idempotency(), paymentHandler.Create)
	router.GET("/api/v1/payments/:id", paymentHandler.GetByID)
	router.GET("/api/v1/payments", paymentHandler.List)
	router.POST("/api/v1/payments/:id/capture", paymentHandler.Capture)
	router.POST("/api/v1/payments/:id/refund", paymentHandler.Refund)

	return router, repo
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

func TestPaymentHandlerCaptureSuccess(t *testing.T) {
	router, repo := setupPaymentRouterWithRepo(t)

	p, _ := payment.NewWithOptions(1000, "BRL", 1, payment.CaptureManual)
	_ = p.MarkAuthorized()
	if err := repo.Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p.ID+"/capture", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var res dto.PaymentResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Status != string(payment.StatusCompleted) {
		t.Fatalf("expected completed, got %s", res.Status)
	}
}

func TestPaymentHandlerCaptureConflict(t *testing.T) {
	router, repo := setupPaymentRouterWithRepo(t)

	p, _ := payment.New(1000, "BRL") // pending, cannot capture
	if err := repo.Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p.ID+"/capture", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestPaymentHandlerRefundSuccess(t *testing.T) {
	router, repo := setupPaymentRouterWithRepo(t)

	p, _ := payment.New(1000, "BRL")
	_ = p.Complete()
	if err := repo.Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{"amount": 400})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p.ID+"/refund", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var res dto.PaymentResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Status != string(payment.StatusPartiallyRefunded) || res.RefundedAmount != 400 {
		t.Fatalf("unexpected refund result: %+v", res)
	}
}

func TestPaymentHandlerRefundFullWithoutBody(t *testing.T) {
	router, repo := setupPaymentRouterWithRepo(t)

	p, _ := payment.New(1000, "BRL")
	_ = p.Complete()
	if err := repo.Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+p.ID+"/refund", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var res dto.PaymentResponse
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Status != string(payment.StatusRefunded) {
		t.Fatalf("expected fully refunded, got %s", res.Status)
	}
}

func TestPaymentHandlerRefundNotFound(t *testing.T) {
	router := setupPaymentRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/missing/refund", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPaymentHandlerRefundInvalidJSON(t *testing.T) {
	router := setupPaymentRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/whatever/refund", bytes.NewReader([]byte(`{invalid`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
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
		nil,
		nil,
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
		nil,
		nil,
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
	h := NewPaymentHandler(nil, nil, nil, nil, nil, nil)

	cases := []struct {
		name       string
		err        error
		statusCode int
	}{
		{"not found", payment.ErrNotFound, http.StatusNotFound},
		{"invalid amount", payment.ErrInvalidAmount, http.StatusBadRequest},
		{"invalid installments", payment.ErrInvalidInstallments, http.StatusBadRequest},
		{"refund exceeds", payment.ErrRefundExceedsAmount, http.StatusBadRequest},
		{"invalid transition", payment.ErrInvalidTransition, http.StatusConflict},
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
