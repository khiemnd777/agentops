package main

import (
	"context"
	"log"
	"os"

	"agentops-workspace/api/internal/app"
	"agentops-workspace/api/internal/config"
	"agentops-workspace/api/internal/db"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()
	migrationsDir := "migrations"
	if _, err := os.Stat(migrationsDir); err != nil {
		migrationsDir = "/app/migrations"
	}
	if err := db.Migrate(ctx, pool, migrationsDir); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
	server, err := app.New(ctx, cfg, pool)
	if err != nil {
		log.Fatalf("build app: %v", err)
	}
	log.Printf("AgentOps Workspace API listening on %s", cfg.APIAddr)
	if err := server.Listen(cfg.APIAddr); err != nil {
		log.Fatal(err)
	}
}
