package main

import (
        "context"
        "fmt"
        "log"
        "os"
        "path/filepath"
        "strings"

        "github.com/code-guardian/code-guardian/internal/config"
        "github.com/code-guardian/code-guardian/internal/db"
        "github.com/code-guardian/code-guardian/internal/rag"
)

func main() {
        cfg := config.Load()
        ctx := context.Background()
        pool, err := db.Connect(ctx, cfg.DatabaseURL)
        if err != nil {
                log.Fatalf("database: %v", err)
        }
        defer pool.Close()
        if err := db.Migrate(ctx, pool); err != nil {
                log.Fatalf("migrate: %v", err)
        }

        entries, err := os.ReadDir(cfg.GuidelinesDir)
        if err != nil {
                log.Fatalf("guidelines: %v", err)
        }
        for _, entry := range entries {
                if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
                        continue
                }
                slug := strings.TrimSuffix(entry.Name(), ".md")
                raw, err := os.ReadFile(filepath.Join(cfg.GuidelinesDir, entry.Name()))
                if err != nil {
                        log.Fatalf("read %s: %v", entry.Name(), err)
                }
                content := string(raw)
                title := slug
                if line, _, ok := strings.Cut(content, "\n"); ok {
                        title = strings.TrimPrefix(strings.TrimSpace(line), "# ")
                }
                if err := rag.UpsertGuideline(ctx, pool, slug, title, content); err != nil {
                        log.Fatalf("seed %s: %v", slug, err)
                }
                fmt.Printf("Seeded guideline %s\n", slug)
        }
}
