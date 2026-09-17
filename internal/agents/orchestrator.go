package agents

import (
        "context"
        "fmt"
        "strings"
        "sync"

        "github.com/code-guardian/code-guardian/internal/domain"
        "github.com/code-guardian/code-guardian/internal/llm"
        "github.com/code-guardian/code-guardian/internal/rag"
        "github.com/jackc/pgx/v5/pgxpool"
)

type Orchestrator struct {
        pool *pgxpool.Pool
        llm  *llm.Client
}

func New(pool *pgxpool.Pool, client *llm.Client) *Orchestrator {
        return &Orchestrator{pool: pool, llm: client}
}

func clipDiff(diff string) string {
        const max = 24000
        if len(diff) <= max {
                return diff
        }
        return diff[:max] + "\n\n[diff truncated for token budget]"
}

// Run is the Go agent loop (same shape as LangGraph, without the JS SDK):
// retrieve guidelines -> linter + security in parallel -> editor aggregates.
func (o *Orchestrator) Run(ctx context.Context, pr domain.PullRequestContext) (domain.ReviewResult, error) {
        provider, model := o.llm.Info()
        query := pr.Title + "\n" + strings.Join(pr.Files, "\n") + "\n" + truncate(pr.Diff, 4000)
        hits, err := rag.SearchGuidelines(ctx, o.pool, query, 4)
        if err != nil {
                return domain.ReviewResult{}, fmt.Errorf("rag: %w", err)
        }
        guidelines := rag.FormatGuidelines(hits)
        payload := fmt.Sprintf(
                "Guidelines:\n%s\n\nPR: %s/%s#%d %s\nFiles:\n%s\n\nDiff:\n%s",
                guidelines, pr.Owner, pr.Repo, pr.PRNumber, pr.Title, strings.Join(pr.Files, "\n"), clipDiff(pr.Diff),
        )

        var (
                wg       sync.WaitGroup
                linter   llm.ChatResult
                security llm.ChatResult
                lErr     error
                sErr     error
        )
        wg.Add(2)
        go func() {
                defer wg.Done()
                linter, lErr = o.llm.Run(ctx, domain.AgentLinter,
                        "You are the Linter Agent for Code Guardian. Find architectural smells, missing error handling, concurrency issues, and style violations. Use the company guidelines. Return a bullet list. No exploit steps.",
                        payload)
        }()
        go func() {
                defer wg.Done()
                security, sErr = o.llm.Run(ctx, domain.AgentSecurity,
                        "You are the Security Agent for Code Guardian. Flag injection, auth gaps, secret leaks, XSS, SSRF, and unsafe shell/SQL. Map each finding to a guideline when possible. Describe impact and a safe fix. Never provide exploit PoCs or attack payloads.",
                        payload)
        }()
        wg.Wait()
        if lErr != nil {
                return domain.ReviewResult{}, lErr
        }
        if sErr != nil {
                return domain.ReviewResult{}, sErr
        }

        editor, err := o.llm.Run(ctx, domain.AgentEditor,
                "You are the Editor Agent. Aggregate linter and security findings into one GitHub markdown comment. Sections: Summary, Architecture, Security, Guideline alignment, Residual risk. Deduplicate. Severity: high/medium/low. Be specific to the diff.",
                fmt.Sprintf("Guidelines:\n%s\n\nLinter findings:\n%s\n\nSecurity findings:\n%s\n\nPR title: %s\nFiles: %s",
                        guidelines, linter.Text, security.Text, pr.Title, strings.Join(pr.Files, ", ")))
        if err != nil {
                return domain.ReviewResult{}, err
        }

        return domain.ReviewResult{
                CommentMarkdown: editor.Text,
                Metrics:         []domain.LlmCallMetric{linter.Metric, security.Metric, editor.Metric},
                Provider:        provider,
                Model:           model,
        }, nil
}

func truncate(s string, n int) string {
        if len(s) <= n {
                return s
        }
        return s[:n]
}
