package roblox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResolveUserID_NumericID(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	id, username, err := client.ResolveUserID(ctx, "123456")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if id != 123456 {
		t.Errorf("Expected ID 123456, got %d", id)
	}
	if username != "123456" {
		t.Errorf("Expected username '123456', got %s", username)
	}
}

func TestResolveUserID_Empty(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	_, _, err := client.ResolveUserID(ctx, "   ")
	if err == nil {
		t.Errorf("Expected error for empty string query, got nil")
	}
}

func TestLookupUser_Mock(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/usernames/users":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":[{"requestedUsername":"TestUser","id":100,"name":"TestUser"}]}`))
		case "/v1/users/100":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"description":"Hello World","created":"2020-01-01T00:00:00Z","isBanned":false,"name":"TestUser","displayName":"Tester"}`))
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"count": 42, "data": []}`))
		}
	}))
	defer mockServer.Close()

	client := NewClient()
	client.httpClient = mockServer.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Direct numeric resolution test
	id, name, err := client.ResolveUserID(ctx, "100")
	if err != nil {
		t.Fatalf("Unexpected error resolving user id: %v", err)
	}
	if id != 100 || name != "100" {
		t.Errorf("Expected 100, got %d / %s", id, name)
	}
}
