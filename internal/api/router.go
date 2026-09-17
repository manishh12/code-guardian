package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/code-guardian/code-guardian/internal/agents"
	"github.com/code-guardian/code-guardian/internal/config"
	"github.com/code-guardian/code-guardian/internal/domain"
	"github.com/code-guardian/code-guardian/internal/githubapp"
	"github.com/code-guardian/code-guardian/internal/llm"
	"github.com/code-guardian/code-guardian/internal/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	cfg     config.Config
	orch    *agents.Orchestrator
	metrics *metrics.Store
	github  *githubapp.Client
	llm     *llm.Client
}

func NewRouter(cfg config.Config, pool *pgxpool.Pool) http.Handler {
	client := llm.New(cfg)
	s := &Server{
		cfg:     cfg,
		orch:    agents.New(pool, client),
		metrics: metrics.NewStore(pool, cfg),
		github:  githubapp.NewClient(cfg.GitHubToken),
		llm:     client,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /api/metrics", s.listMetrics)
	mux.HandleFunc("POST /webhooks/github", s.githubWebhook)
	mux.HandleFunc("POST /webhooks/demo", s.demoReview)
	mux.Handle("GET /", spaHandler(cfg.StaticDir))
	return withCORS(mux)
}

func spaHandler(staticDir string) http.Handler {
	files := http.FileServer(http.Dir(staticDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := filepath.Join(staticDir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(requested); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}

		index := filepath.Join(staticDir, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.Error(w, "dashboard has not been built", http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, index)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Hub-Signature-256, X-GitHub-Event, X-GitHub-Delivery")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"provider": s.cfg.ResolveProvider(),
		"backend":  "go",
	})
}

func (s *Server) listMetrics(w http.ResponseWriter, r *http.Request) {
	payload, err := s.metrics.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	provider, model := s.llm.Info()
	payload["provider"] = map[string]string{"provider": provider, "model": model}
	payload["resolvedProvider"] = provider
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) executeReview(ctx context.Context, pr domain.PullRequestContext, deliveryID string) (string, domain.ReviewResult, error) {
	provider, model := s.llm.Info()
	id := metrics.NewID()
	if err := s.metrics.Create(ctx, id, deliveryID, pr.Owner+"/"+pr.Repo, pr.PRNumber, provider, model); err != nil {
		return "", domain.ReviewResult{}, err
	}
	result, err := s.orch.Run(ctx, pr)
	if err != nil {
		_ = s.metrics.Finish(ctx, id, nil, err)
		return id, domain.ReviewResult{}, err
	}
	if err := s.metrics.Finish(ctx, id, &result, nil); err != nil {
		return id, result, err
	}
	tokens := 0
	for _, m := range result.Metrics {
		tokens += m.TokensIn + m.TokensOut
	}
	body := result.CommentMarkdown + "\n\n---\n_Code Guardian · " + result.Provider + "/" + result.Model + " · " + strconv.Itoa(tokens) + " tokens_"
	if s.github != nil {
		if err := s.github.PostComment(ctx, pr, body); err != nil {
			log.Printf("github comment: %v", err)
		}
	}
	return id, result, nil
}

func (s *Server) demoReview(w http.ResponseWriter, r *http.Request) {
	delivery := fmt.Sprintf("demo-%d", time.Now().UnixNano())
	id, result, err := s.executeReview(r.Context(), githubapp.DemoPullRequest(), delivery)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"reviewId": id,
		"comment":  result.CommentMarkdown,
		"metrics":  result.Metrics,
	})
}

func (s *Server) githubWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body"})
		return
	}
	if !githubapp.VerifySignature(s.cfg.GitHubWebhookSecret, body, r.Header.Get("X-Hub-Signature-256")) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
		return
	}
	event := r.Header.Get("X-GitHub-Event")
	delivery := r.Header.Get("X-GitHub-Delivery")
	if event == "ping" {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if event != "pull_request" {
		writeJSON(w, http.StatusAccepted, map[string]any{"ignored": event})
		return
	}
	var payload struct {
		Action     string `json:"action"`
		Number     int    `json:"number"`
		Repository struct {
			Name  string `json:"name"`
			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repository"`
		PullRequest struct {
			Number int `json:"number"`
		} `json:"pull_request"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	switch payload.Action {
	case "opened", "reopened", "synchronize":
	default:
		writeJSON(w, http.StatusAccepted, map[string]any{"ignored": payload.Action})
		return
	}
	owner := payload.Repository.Owner.Login
	repo := payload.Repository.Name
	prNumber := payload.PullRequest.Number
	if prNumber == 0 {
		prNumber = payload.Number
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "delivery": delivery})

	go func() {
		ctx := context.Background()
		if s.github == nil {
			log.Printf("No GITHUB_TOKEN; cannot fetch PR diffs")
			return
		}
		pr, err := s.github.LoadPullRequest(ctx, owner, repo, prNumber)
		if err != nil {
			log.Printf("load pr: %v", err)
			return
		}
		pr.DeliveryID = delivery
		if _, _, err := s.executeReview(ctx, pr, delivery); err != nil {
			log.Printf("review failed: %v", err)
		}
	}()
}
