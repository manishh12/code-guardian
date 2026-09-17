package metrics

import (
        "context"

        "github.com/code-guardian/code-guardian/internal/config"
        "github.com/code-guardian/code-guardian/internal/domain"
        "github.com/google/uuid"
        "github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
        pool *pgxpool.Pool
        cfg  config.Config
}

func NewStore(pool *pgxpool.Pool, cfg config.Config) *Store {
        return &Store{pool: pool, cfg: cfg}
}

func (s *Store) PaidEquivalent(tokensIn, tokensOut int) float64 {
        return (float64(tokensIn)/1_000_000)*s.cfg.OpenAIInputUSDPer1M +
                (float64(tokensOut)/1_000_000)*s.cfg.OpenAIOutputUSDPer1M
}

func NewID() string {
        return uuid.NewString()
}

func (s *Store) Create(ctx context.Context, id, deliveryID, repo string, prNumber int, provider, model string) error {
        if deliveryID == "" {
                deliveryID = id
        }
        _, err := s.pool.Exec(ctx, `
                INSERT INTO reviews (id, delivery_id, repo, pr_number, status, provider, model)
                VALUES ($1, $2, $3, $4, 'running', $5, $6)
                ON CONFLICT (delivery_id) DO NOTHING`, id, deliveryID, repo, prNumber, provider, model)
        return err
}

func (s *Store) Finish(ctx context.Context, id string, result *domain.ReviewResult, fail error) error {
        tokensIn, tokensOut, latency := 0, 0, 0
        var comment *string
        provider, model := (*string)(nil), (*string)(nil)
        status := "completed"
        var errMsg *string
        if fail != nil {
                status = "failed"
                m := fail.Error()
                errMsg = &m
        }
        if result != nil {
                for _, m := range result.Metrics {
                        tokensIn += m.TokensIn
                        tokensOut += m.TokensOut
                        latency += m.LatencyMs
                }
                comment = &result.CommentMarkdown
                provider = &result.Provider
                model = &result.Model
        }
        equivalent := s.PaidEquivalent(tokensIn, tokensOut)
        actual := 0.0
        _, err := s.pool.Exec(ctx, `
                UPDATE reviews SET
                        status = $2,
                        tokens_in = $3,
                        tokens_out = $4,
                        latency_ms = $5,
                        estimated_cost_usd = $6,
                        equivalent_paid_usd = $7,
                        comment_md = $8,
                        error = $9,
                        provider = COALESCE($10, provider),
                        model = COALESCE($11, model)
                WHERE id = $1`,
                id, status, tokensIn, tokensOut, latency, actual, equivalent, comment, errMsg, provider, model)
        if err != nil {
                return err
        }
        if result == nil {
                return nil
        }
        for _, m := range result.Metrics {
                if _, err := s.pool.Exec(ctx, `
                        INSERT INTO llm_calls (review_id, agent, tokens_in, tokens_out, latency_ms)
                        VALUES ($1, $2, $3, $4, $5)`, id, m.Agent, m.TokensIn, m.TokensOut, m.LatencyMs); err != nil {
                        return err
                }
        }
        return nil
}

type Totals struct {
        ReviewCount       int     `json:"review_count"`
        Tokens            int     `json:"tokens"`
        AvgLatencyMs      int     `json:"avg_latency_ms"`
        ActualCostUSD     float64 `json:"actual_cost_usd"`
        EquivalentPaidUSD float64 `json:"equivalent_paid_usd"`
}

type ReviewRow struct {
        ID                string  `json:"id"`
        Repo              string  `json:"repo"`
        PRNumber          int     `json:"pr_number"`
        Status            string  `json:"status"`
        Provider          string  `json:"provider"`
        Model             string  `json:"model"`
        TokensIn          int     `json:"tokens_in"`
        TokensOut         int     `json:"tokens_out"`
        LatencyMs         int     `json:"latency_ms"`
        EstimatedCostUSD  string  `json:"estimated_cost_usd"`
        EquivalentPaidUSD string  `json:"equivalent_paid_usd"`
        CreatedAt         string  `json:"created_at"`
        Error             *string `json:"error"`
}

type AgentRow struct {
        Agent        string `json:"agent"`
        Tokens       int    `json:"tokens"`
        AvgLatencyMs int    `json:"avg_latency_ms"`
}

func (s *Store) List(ctx context.Context) (map[string]any, error) {
        rows, err := s.pool.Query(ctx, `
                SELECT id::text, repo, pr_number, status, provider, model, tokens_in, tokens_out,
                       latency_ms, estimated_cost_usd::text, equivalent_paid_usd::text, created_at::text, error
                FROM reviews ORDER BY created_at DESC LIMIT 50`)
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        var reviews []ReviewRow
        for rows.Next() {
                var r ReviewRow
                if err := rows.Scan(&r.ID, &r.Repo, &r.PRNumber, &r.Status, &r.Provider, &r.Model,
                        &r.TokensIn, &r.TokensOut, &r.LatencyMs, &r.EstimatedCostUSD, &r.EquivalentPaidUSD, &r.CreatedAt, &r.Error); err != nil {
                        return nil, err
                }
                reviews = append(reviews, r)
        }
        if reviews == nil {
                reviews = []ReviewRow{}
        }

        var totals Totals
        if err := s.pool.QueryRow(ctx, `
                SELECT COUNT(*)::int,
                       COALESCE(SUM(tokens_in + tokens_out), 0)::int,
                       COALESCE(AVG(latency_ms), 0)::int,
                       COALESCE(SUM(estimated_cost_usd), 0)::float8,
                       COALESCE(SUM(equivalent_paid_usd), 0)::float8
                FROM reviews`).Scan(
                &totals.ReviewCount, &totals.Tokens, &totals.AvgLatencyMs, &totals.ActualCostUSD, &totals.EquivalentPaidUSD,
        ); err != nil {
                return nil, err
        }

        agentRows, err := s.pool.Query(ctx, `
                SELECT agent, COALESCE(SUM(tokens_in + tokens_out), 0)::int,
                       COALESCE(AVG(latency_ms), 0)::int
                FROM llm_calls GROUP BY agent ORDER BY agent`)
        if err != nil {
                return nil, err
        }
        defer agentRows.Close()
        var byAgent []AgentRow
        for agentRows.Next() {
                var a AgentRow
                if err := agentRows.Scan(&a.Agent, &a.Tokens, &a.AvgLatencyMs); err != nil {
                        return nil, err
                }
                byAgent = append(byAgent, a)
        }
        if byAgent == nil {
                byAgent = []AgentRow{}
        }

        return map[string]any{
                "totals":  totals,
                "reviews": reviews,
                "byAgent": byAgent,
        }, nil
}
