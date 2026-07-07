package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIdempotencyMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing key", func(t *testing.T) {
		router := gin.New()
		router.POST("/payments", Idempotency(), func(c *gin.Context) {
			c.Status(http.StatusCreated)
		})

		req := httptest.NewRequest(http.MethodPost, "/payments", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("sets key in context", func(t *testing.T) {
		router := gin.New()
		router.POST("/payments", Idempotency(), func(c *gin.Context) {
			key, ok := c.Get("idempotency_key")
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "missing key"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"key": key})
		})

		req := httptest.NewRequest(http.MethodPost, "/payments", nil)
		req.Header.Set("Idempotency-Key", "key-123")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
		}
	})
}
