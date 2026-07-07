package middleware

import "github.com/gin-gonic/gin"

func Idempotency() gin.HandlerFunc {

	return func(c *gin.Context) {

		key := c.GetHeader("Idempotency-Key")

		if key == "" {
			c.AbortWithStatusJSON(400, gin.H{
				"error": "Idempotency-Key is required",
			})
			return
		}

		c.Set("idempotency_key", key)

		c.Next()
	}

}
