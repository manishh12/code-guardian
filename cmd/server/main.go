package main

import (
        "context"
        "log"
        "net/http"
        "os"
        "os/signal"
        "syscall"
        "time"

        "github.com/code-guardian/code-guardian/internal/api"
        "github.com/code-guardian/code-guardian/internal/config"
        "github.com/code-guardian/code-guardian/internal/db"
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

        server := &http.Server{
                Addr:              ":" + cfg.Port,
                Handler:           api.NewRouter(cfg, pool),
                ReadHeaderTimeout: 10 * time.Second,
        }

        go func() {
                log.Printf("Code Guardian API on http://localhost:%s (%s)", cfg.Port, cfg.ResolveProvider())
                if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                        log.Fatalf("listen: %v", err)
                }
        }()

        stop := make(chan os.Signal, 1)
        signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
        <-stop

        shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        _ = server.Shutdown(shutdownCtx)
}
