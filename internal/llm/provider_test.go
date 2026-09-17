package llm

import (
	"context"
	"testing"

	"github.com/code-guardian/code-guardian/internal/config"
	"github.com/code-guardian/code-guardian/internal/domain"
)

func TestMockProviderReturnsEditorMarkdown(t *testing.T) {
	client := New(config.Config{LLMProvider: config.ProviderMock})
	result, err := client.Run(context.Background(), domain.AgentEditor, "sys", "user")
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider != "mock" {
		t.Fatalf("provider=%s", result.Provider)
	}
	if result.Metric.TokensIn < 1 || result.Metric.TokensOut < 1 {
		t.Fatal("expected token estimates")
	}
	if result.Text == "" {
		t.Fatal("empty mock comment")
	}
}

func TestResolveFallsBackToMockWithoutGroqKey(t *testing.T) {
	cfg := config.Config{LLMProvider: config.ProviderGroq, GroqAPIKey: ""}
	if cfg.ResolveProvider() != config.ProviderMock {
		t.Fatal("missing groq key should use mock")
	}
}
