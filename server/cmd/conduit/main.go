package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"

	"github.com/arTECRAD/conduit/server/internal/config"
	"github.com/arTECRAD/conduit/server/internal/handler"
	"github.com/arTECRAD/conduit/server/internal/middleware"
	mqttpkg "github.com/arTECRAD/conduit/server/internal/mqtt"
	"github.com/arTECRAD/conduit/server/internal/repository"
	"github.com/arTECRAD/conduit/server/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Database
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	// Migrations
	if cfg.RunMigrations {
		if err := runMigrations(cfg.DatabaseURL); err != nil {
			return fmt.Errorf("run migrations: %w", err)
		}
	}

	// Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("parse Redis URL: %w", err)
	}
	rdb := redis.NewClient(redisOpts)
	defer func() { _ = rdb.Close() }()

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("ping Redis: %w", err)
	}

	// MQTT
	mqttClient, err := mqttpkg.NewClient(cfg.MQTTBrokerURL, cfg.MQTTUsername, cfg.MQTTPassword, cfg.TLSCACertPath)
	if err != nil {
		// Non-fatal in development: continue without MQTT
		slog.Warn("MQTT connection failed, continuing without MQTT", "err", err)
		mqttClient = nil
	}

	// Repositories
	q := repository.New(pool)
	telemetryRepo := repository.NewSensorReadingsCustom(pool)
	commander := mqttpkg.NewCommander(mqttClient)

	// Services
	authSvc := service.NewAuthService(q, cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL, cfg.BcryptCost)
	deviceSvc := service.NewDeviceService(q, commander, cfg.BcryptCost)
	telemetrySvc := service.NewTelemetryService(telemetryRepo, q)

	// MQTT Ingester (optional)
	var ingester *mqttpkg.Ingester
	if mqttClient != nil {
		ingester, err = mqttpkg.NewIngester(mqttClient, telemetryRepo)
		if err != nil {
			slog.Warn("MQTT ingester setup failed", "err", err)
		}
	}

	// Handlers
	authHandler := handler.NewAuthHandler(authSvc, cfg.RefreshTokenTTL, cfg.Env != "development")
	deviceHandler := handler.NewDeviceHandler(deviceSvc)
	telemetryHandler := handler.NewTelemetryHandler(telemetrySvc)
	userHandler := handler.NewUserHandler(authSvc)

	// JWT validator function
	jwtValidateFn := func(token string) (string, error) {
		userID, err := authSvc.ValidateAccessToken(token)
		if err != nil {
			return "", err
		}
		return userID.String(), nil
	}

	// Echo
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Global middleware
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.RequestID())
	e.Use(middleware.RequestLogger())
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins:     []string{cfg.CORSOrigin},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderAuthorization, echo.HeaderContentType},
		AllowCredentials: true,
	}))

	// Routes
	api := e.Group("/api/v1")

	// Auth routes
	authGroup := api.Group("/auth")
	authRateLimit := middleware.RateLimiter(rdb, middleware.IPKeyFn, 20, time.Minute)
	authGroup.Use(authRateLimit)
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)
	authGroup.POST("/logout", authHandler.Logout)

	// Device registration (provisioning token auth)
	api.POST("/devices/register", deviceHandler.Register, middleware.DeviceAuth(cfg.ProvisioningToken))

	// Protected routes (JWT auth)
	jwtMiddleware := middleware.JWTAuth(jwtValidateFn)
	protected := api.Group("", jwtMiddleware)
	protected.POST("/devices/claim", deviceHandler.Claim)
	protected.GET("/devices", deviceHandler.List)
	protected.GET("/devices/:id", deviceHandler.Get)
	protected.PATCH("/devices/:id", deviceHandler.Update)
	protected.DELETE("/devices/:id", deviceHandler.Delete)
	protected.GET("/devices/:id/telemetry", telemetryHandler.Get)
	protected.GET("/users/me", userHandler.GetProfile)

	// Start server
	serverAddr := fmt.Sprintf(":%d", cfg.ServerPort)
	slog.Info("starting server", "addr", serverAddr, "env", cfg.Env)

	go func() {
		if err := e.Start(serverAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "err", err)
	}

	if mqttClient != nil {
		mqttClient.Disconnect(2000)
	}
	if ingester != nil {
		ingester.Stop()
	}

	slog.Info("server stopped")
	return nil
}

func runMigrations(databaseURL string) error {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	slog.Info("migrations applied")
	return nil
}
