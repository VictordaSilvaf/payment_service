package webhook

import (
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Subscription é o registro de um endpoint externo (lojista) interessado em um
// tipo de evento. Quando o evento ocorre, o serviço envia um POST assinado para
// a URL cadastrada.
type Subscription struct {
	ID        string
	URL       string
	Secret    string
	EventType string
	Active    bool
	CreatedAt time.Time
}

// NewSubscription valida os dados e cria uma assinatura ativa. Se o secret vier
// vazio, gera um automaticamente — o lojista usa esse segredo para validar a
// assinatura HMAC das entregas.
func NewSubscription(rawURL, secret, eventType string) (*Subscription, error) {
	if !isValidURL(rawURL) {
		return nil, ErrInvalidURL
	}
	if strings.TrimSpace(eventType) == "" {
		return nil, ErrInvalidEventType
	}
	if strings.TrimSpace(secret) == "" {
		secret = uuid.NewString()
	}

	return &Subscription{
		ID:        uuid.NewString(),
		URL:       rawURL,
		Secret:    secret,
		EventType: eventType,
		Active:    true,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func isValidURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
