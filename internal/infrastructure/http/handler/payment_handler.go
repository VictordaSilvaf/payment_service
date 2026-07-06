package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"payment_service/internal/application/dto"
	"payment_service/internal/application/usecase"
	"payment_service/internal/domain/payment"
)

type PaymentHandler struct {
	createPayment *usecase.CreatePayment
	getPayment    *usecase.GetPayment
	listPayment   *usecase.ListPayment
}

func NewPaymentHandler(createPayment *usecase.CreatePayment, getPayment *usecase.GetPayment, listPayment *usecase.ListPayment) *PaymentHandler {
	return &PaymentHandler{
		createPayment: createPayment,
		getPayment:    getPayment,
		listPayment:   listPayment,
	}
}

func (h *PaymentHandler) Create(c *gin.Context) {
	var req dto.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.createPayment.Execute(c.Request.Context(), req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, result)
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
		c.Query("search"),
	)
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
	case errors.Is(err, payment.ErrInvalidAmount):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
