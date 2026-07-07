package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// Sign gera a assinatura HMAC-SHA256 do corpo usando o segredo da assinatura.
// O lojista recomputa o mesmo HMAC com o segredo que possui e compara, garantindo
// autenticidade (veio de nós) e integridade (não foi alterado no caminho).
// Formato: "sha256=<hex>".
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
