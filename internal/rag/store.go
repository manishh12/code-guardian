package rag

import (
        "context"
        "fmt"
        "strings"

        "github.com/jackc/pgx/v5/pgxpool"
)

type GuidelineHit struct {
        Slug     string  `json:"slug"`
        Title    string  `json:"title"`
        Content  string  `json:"content"`
        Distance float64 `json:"distance"`
}

func UpsertGuideline(ctx context.Context, pool *pgxpool.Pool, slug, title, content string) error {
        emb := VectorLiteral(EmbedText(title + "\n" + content))
        _, err := pool.Exec(ctx, `
                INSERT INTO guidelines (slug, title, content, embedding)
                VALUES ($1, $2, $3, $4::vector)
                ON CONFLICT (slug) DO UPDATE SET
                        title = EXCLUDED.title,
                        content = EXCLUDED.content,
                        embedding = EXCLUDED.embedding,
                        updated_at = now()`, slug, title, content, emb)
        return err
}

func SearchGuidelines(ctx context.Context, pool *pgxpool.Pool, query string, limit int) ([]GuidelineHit, error) {
        emb := VectorLiteral(EmbedText(query))
        rows, err := pool.Query(ctx, `
                SELECT slug, title, content, (embedding <=> $1::vector) AS distance
                FROM guidelines
                ORDER BY embedding <=> $1::vector
                LIMIT $2`, emb, limit)
        if err != nil {
                return nil, err
        }
        defer rows.Close()

        var hits []GuidelineHit
        for rows.Next() {
                var hit GuidelineHit
                if err := rows.Scan(&hit.Slug, &hit.Title, &hit.Content, &hit.Distance); err != nil {
                        return nil, err
                }
                hits = append(hits, hit)
        }
        return hits, rows.Err()
}

func FormatGuidelines(hits []GuidelineHit) string {
        if len(hits) == 0 {
                return "No internal guidelines were retrieved. Review using general engineering practice and say when a rule is assumed."
        }
        parts := make([]string, 0, len(hits))
        for _, hit := range hits {
                parts = append(parts, fmt.Sprintf("### %s (%s, distance=%.3f)\n%s", hit.Title, hit.Slug, hit.Distance, hit.Content))
        }
        return strings.Join(parts, "\n\n")
}
