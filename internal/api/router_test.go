package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/code-guardian/code-guardian/internal/config"
)

func TestHealthAndWebhookAuth(t *testing.T) {
	s := &Server{cfg: config.Config{LLMProvider: config.ProviderMock, GitHubWebhookSecret: "change-me"}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /webhooks/github", s.githubWebhook)

	health := httptest.NewRecorder()
	mux.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health=%d", health.Code)
	}

	body := []byte(`{"action":"opened"}`)
	bad := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256=00")
	req.Header.Set("X-GitHub-Event", "ping")
	mux.ServeHTTP(bad, req)
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", bad.Code)
	}

	mac := hmac.New(sha256.New, []byte("change-me"))
	mac.Write(body)
	ok := httptest.NewRecorder()
	good := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(body))
	good.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	good.Header.Set("X-GitHub-Event", "ping")
	mux.ServeHTTP(ok, good)
	if ok.Code != http.StatusOK {
		t.Fatalf("ping=%d body=%s", ok.Code, ok.Body.String())
	}
}

func TestSPAHandlerServesDashboardAndFallback(t *testing.T) {
	dir := t.TempDir()
	index := []byte("<html><body>dashboard</body></html>")
	if err := os.WriteFile(filepath.Join(dir, "index.html"), index, 0o600); err != nil {
		t.Fatal(err)
	}

	handler := spaHandler(dir)
	for _, path := range []string{"/", "/reviews/example"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s returned %d", path, recorder.Code)
		}
		if recorder.Body.String() != string(index) {
			t.Fatalf("%s did not return the dashboard", path)
		}
	}
}
