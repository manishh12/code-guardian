package db

import (
        "context"

        "github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
        return pgxpool.New(ctx, databaseURL)
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
        stmts := []string{
                `CREATE EXTENSION IF NOT EXISTS vector`,
                `CREATE TABLE IF NOT EXISTS guidelines (
                        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                        slug TEXT UNIQUE NOT NULL,
                        title TEXT NOT NULL,
                        content TEXT NOT NULL,
                        embedding vector(384) NOT NULL,
                        updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
                )`,
                `CREATE TABLE IF NOT EXISTS reviews (
                        id UUID PRIMARY KEY,
                        delivery_id TEXT UNIQUE,
                        repo TEXT NOT NULL,
                        pr_number INTEGER NOT NULL,
                        status TEXT NOT NULL,
                        provider TEXT NOT NULL,
                        model TEXT NOT NULL,
                        tokens_in INTEGER NOT NULL DEFAULT 0,
                        tokens_out INTEGER NOT NULL DEFAULT 0,
                        latency_ms INTEGER NOT NULL DEFAULT 0,
                        estimated_cost_usd NUMERIC(12, 6) NOT NULL DEFAULT 0,
                        equivalent_paid_usd NUMERIC(12, 6) NOT NULL DEFAULT 0,
                        comment_md TEXT,
                        error TEXT,
                        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
                )`,
                `CREATE TABLE IF NOT EXISTS llm_calls (
                        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                        review_id UUID REFERENCES reviews(id) ON DELETE CASCADE,
                        agent TEXT NOT NULL,
                        tokens_in INTEGER NOT NULL DEFAULT 0,
                        tokens_out INTEGER NOT NULL DEFAULT 0,
                        latency_ms INTEGER NOT NULL DEFAULT 0,
                        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
                )`,
        }
        for _, stmt := range stmts {
                if _, err := pool.Exec(ctx, stmt); err != nil {
                        return err
                }
        }
        return nil
}
