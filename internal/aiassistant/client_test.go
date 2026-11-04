package aiassistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnalyzeSendsServiceTokenAndAcceptsAssistOnlyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/ai/analyze" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-AI-Service-Token") != "private-token" {
			t.Fatal("service token missing")
		}
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got["feature"] != "anti_flood" || got["text"] != "repeat promo" {
			t.Fatalf("unexpected request payload: %#v", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"mode":"assist","enforcement_enabled":false,"analysis":{"category":"spam","confidence":0.9,"recommendation":"admin_review","rationale":"Repeated promotion."}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "private-token")
	got, err := client.Analyze(context.Background(), -100, 12, 99, "anti_flood", " repeat promo ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Category != "spam" || got.Recommendation != "admin_review" {
		t.Fatalf("unexpected analysis: %+v", got)
	}
}

func TestNewClientRequiresURLAndToken(t *testing.T) {
	if NewClient("", "secret") != nil || NewClient("http://localhost:8000", "") != nil {
		t.Fatal("client should be disabled unless URL and service token are configured")
	}
}

func TestAnalyzeRejectsPunitiveResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"mode":"assist","enforcement_enabled":false,"analysis":{"category":"spam","confidence":1,"recommendation":"ban","rationale":"bad"}}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, "secret")
	if _, err := client.Analyze(context.Background(), 1, 2, 3, "anti_flood", "spam"); err == nil {
		t.Fatal("punitive recommendation must be rejected")
	}
}
