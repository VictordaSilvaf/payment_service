package webhookclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"payment_service/internal/domain/webhook"
)

func TestClientSendsHeadersAndBody(t *testing.T) {
	var (
		gotSig   string
		gotID    string
		gotEvent string
		gotBody  string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Webhook-Signature")
		gotID = r.Header.Get("X-Webhook-Id")
		gotEvent = r.Header.Get("X-Webhook-Event")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	client := New(2 * time.Second)
	status, err := client.Send(context.Background(), webhook.SendRequest{
		URL:       srv.URL,
		EventType: "payment.completed",
		WebhookID: "wid-1",
		Signature: "sha256=abc",
		Body:      []byte(`{"id":"p1"}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", status)
	}
	if gotSig != "sha256=abc" || gotID != "wid-1" || gotEvent != "payment.completed" {
		t.Fatalf("unexpected headers: sig=%s id=%s event=%s", gotSig, gotID, gotEvent)
	}
	if gotBody != `{"id":"p1"}` {
		t.Fatalf("unexpected body: %s", gotBody)
	}
}

func TestClientReturnsErrorOnUnreachableHost(t *testing.T) {
	client := New(200 * time.Millisecond)
	_, err := client.Send(context.Background(), webhook.SendRequest{
		URL:  "http://127.0.0.1:0",
		Body: []byte(`{}`),
	})
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}
}
