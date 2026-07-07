package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"payment_service/internal/application/dto"
	"payment_service/internal/application/idempotency"
	"payment_service/internal/application/usecase"
	"payment_service/internal/domain/payment"
)

type PaymentHandler struct {
	createPayment  *usecase.CreatePayment
	getPayment     *usecase.GetPayment
	listPayment    *usecase.ListPayment
	capturePayment *usecase.CapturePayment
	refundPayment  *usecase.RefundPayment
	idempotency    *idempotency.Service
}

func NewPaymentHandler(
	createPayment *usecase.CreatePayment,
	getPayment *usecase.GetPayment,
	listPayment *usecase.ListPayment,
	capturePayment *usecase.CapturePayment,
	refundPayment *usecase.RefundPayment,
	idempotencyService *idempotency.Service,
) *PaymentHandler {
	return &PaymentHandler{
		createPayment:  createPayment,
		getPayment:     getPayment,
		listPayment:    listPayment,
		capturePayment: capturePayment,
		refundPayment:  refundPayment,
		idempotency:    idempotencyService,
	}
}

func (h *PaymentHandler) Create(c *gin.Context) {
	var req dto.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key, _ := c.Get("idempotency_key")
	idempotencyKey, _ := key.(string)

	requestHash, err := hashRequest(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	response, err := h.idempotency.Execute(
		c.Request.Context(),
		idempotencyKey,
		requestHash,
		func(ctx context.Context) (idempotency.CachedResponse, error) {
			result, execErr := h.createPayment.Execute(ctx, req)
			if execErr != nil {
				return idempotency.CachedResponse{}, execErr
			}

			body, marshalErr := json.Marshal(result)
			if marshalErr != nil {
				return idempotency.CachedResponse{}, marshalErr
			}

			return idempotency.CachedResponse{
				StatusCode: http.StatusCreated,
				Body:       body,
			}, nil
		},
	)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.Data(response.StatusCode, "application/json", response.Body)
}

func (h *PaymentHandler) GetByID(c *gin.Context) {
	result, err := h.getPayment.Execute(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *PaymentHandler) List(c *gin.Context) {
	result, err := h.listPayment.Execute(
		c.Request.Context(),
		c.Query("page"),
		c.Query("limit"),
		c.Query("sort"),
		c.Query("order"),
		c.Query("status"),
	)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// Capture liquida um pagamento previamente autorizado (fluxo de captura manual).
func (h *PaymentHandler) Capture(c *gin.Context) {
	result, err := h.capturePayment.Execute(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// Refund estorna um pagamento capturado, total ou parcialmente.
func (h *PaymentHandler) Refund(c *gin.Context) {
	var req dto.RefundPaymentRequest
	// O corpo é opcional (estorno total). Só falha se vier um JSON malformado.
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	result, err := h.refundPayment.Execute(c.Request.Context(), c.Param("id"), req.Amount)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *PaymentHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, payment.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, payment.ErrInvalidAmount),
		errors.Is(err, payment.ErrInvalidInstallments),
		errors.Is(err, payment.ErrInvalidCaptureMethod),
		errors.Is(err, payment.ErrInvalidRefundAmount),
		errors.Is(err, payment.ErrRefundExceedsAmount):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, payment.ErrInvalidTransition):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, idempotency.ErrInvalidKey):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, idempotency.ErrAlreadyProcessing):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, idempotency.ErrKeyAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

func hashRequest(req dto.CreatePaymentRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
