package app

import (
	"context"

	"agentops-workspace/api/internal/config"
	"agentops-workspace/api/internal/handlers"
	"agentops-workspace/api/internal/repo"
	"agentops-workspace/api/internal/services"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(ctx context.Context, cfg config.Config, db *pgxpool.Pool) (*fiber.App, error) {
	store := repo.New(db)
	if err := (services.Seeder{Store: store}).Seed(ctx); err != nil {
		return nil, err
	}
	guard := services.NewPathGuard()
	importer := services.NewRunImporter(store, cfg.RunImport.MaxReportsPerScan)
	h := handlers.Handler{
		Store: store, Guard: guard, Scanner: services.RepoScanner{},
		Sync:     services.SyncService{Store: store, Guard: guard},
		Importer: importer,
		Playback: services.PlaybackService{Store: store, Audit: services.AuditService{Store: store}},
	}
	app := fiber.New(fiber.Config{AppName: "AgentOps Workspace API"})
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{AllowOrigins: "*", AllowHeaders: "Origin, Content-Type, Accept, Authorization"}))
	app.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	api := app.Group("/api")
	if cfg.AuthEnabled {
		api.Use(func(c *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusUnauthorized, "auth middleware is configured but no provider is installed in MVP")
		})
	}
	h.Register(api)
	return app, nil
}
