package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestExtractAntigravityTierIDPrefersDefaultAllowedTier(t *testing.T) {
	tierID := extractAntigravityTierID(map[string]any{
		"allowedTiers": []any{
			map[string]any{"id": "free-tier"},
			map[string]any{"id": "g1-pro-tier", "isDefault": true},
		},
		"currentTier": map[string]any{"id": "legacy-tier"},
	})

	if tierID != "g1-pro-tier" {
		t.Fatalf("extractAntigravityTierID() = %q, want %q", tierID, "g1-pro-tier")
	}
}

func TestExtractAntigravityTierIDFallsBackToFirstAllowedTier(t *testing.T) {
	tierID := extractAntigravityTierID(map[string]any{
		"allowedTiers": []any{
			map[string]any{"id": "g1-ultra-tier"},
			map[string]any{"id": "g1-pro-tier"},
		},
	})

	if tierID != "g1-ultra-tier" {
		t.Fatalf("extractAntigravityTierID() = %q, want %q", tierID, "g1-ultra-tier")
	}
}

func TestLoadAntigravityProjectIDOnboardsUsingAllowedTiers(t *testing.T) {
	var onboardTierID atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1internal:loadCodeAssist":
			if got := r.Header.Get("X-Goog-Api-Client"); got != antigravityOAuthAPIClient {
				t.Fatalf("missing X-Goog-Api-Client header: %q", got)
			}
			if got := r.Header.Get("Client-Metadata"); got != antigravityOAuthClientMetadata {
				t.Fatalf("missing Client-Metadata header: %q", got)
			}
			_, _ = w.Write([]byte(`{"allowedTiers":[{"id":"free-tier","isDefault":true}]}`))
		case "/v1internal:onboardUser":
			body, _ := io.ReadAll(r.Body)
			onboardTierID.Store(string(body))
			_, _ = w.Write([]byte(`{"done":true,"response":{"cloudaicompanionProject":{"id":"project-123"}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	projectID, err := loadAntigravityProjectID(context.Background(), server.Client(), server.URL, "token")
	if err != nil {
		t.Fatalf("loadAntigravityProjectID() error = %v", err)
	}
	if projectID != "project-123" {
		t.Fatalf("loadAntigravityProjectID() = %q, want %q", projectID, "project-123")
	}

	payload, _ := onboardTierID.Load().(string)
	if !strings.Contains(payload, `"tierId":"free-tier"`) {
		t.Fatalf("onboardUser payload missing tierId: %s", payload)
	}
}

func TestFetchAntigravityProjectIDWithRetryRetriesTransientFailure(t *testing.T) {
	var loadAttempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1internal:loadCodeAssist":
			attempt := loadAttempts.Add(1)
			if attempt == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":"temporary"}`))
				return
			}
			_, _ = w.Write([]byte(`{"cloudaicompanionProject":"project-456"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalBaseURLs := antigravityOAuthBaseURLs
	antigravityOAuthBaseURLs = []string{server.URL}
	defer func() {
		antigravityOAuthBaseURLs = originalBaseURLs
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	projectID, err := fetchAntigravityProjectIDWithRetry(ctx, server.Client(), "token", 1)
	if err != nil {
		t.Fatalf("fetchAntigravityProjectIDWithRetry() error = %v", err)
	}
	if projectID != "project-456" {
		t.Fatalf("fetchAntigravityProjectIDWithRetry() = %q, want %q", projectID, "project-456")
	}
	if loadAttempts.Load() != 2 {
		t.Fatalf("loadCodeAssist attempts = %d, want %d", loadAttempts.Load(), 2)
	}
}

func TestFetchAntigravityProjectIDWithRetryReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fetchAntigravityProjectIDWithRetry(ctx, &http.Client{}, "token", 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("fetchAntigravityProjectIDWithRetry() error = %v, want context canceled", err)
	}
}
