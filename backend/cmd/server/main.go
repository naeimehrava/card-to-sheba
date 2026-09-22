package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/behnam/card-to-sheba/backend/internal/config"
	"github.com/behnam/card-to-sheba/backend/internal/httpapi"
	"github.com/behnam/card-to-sheba/backend/internal/inquiry"
	"github.com/behnam/card-to-sheba/backend/internal/logging"
	"github.com/behnam/card-to-sheba/backend/internal/storage/sqlite"
	"github.com/behnam/card-to-sheba/backend/internal/zarinhub"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	envFile := flag.String("env-file", ".env", "path to an optional .env file")
	migrateOnly := flag.Bool("migrate-only", false, "apply database migrations and exit")
	flag.Parse()

	cfg, err := config.Load(*envFile)
	if err != nil {
		return err
	}

	logger := logging.New(os.Stdout, cfg.Log.Level, cfg.Log.Format)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := sqlite.Open(ctx, cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}
	defer store.Close()

	if *migrateOnly {
		logger.Info("migrations applied", slog.String("driver", cfg.Database.Driver))
		return nil
	}

	provider := zarinhub.New(zarinhub.Config{
		BaseURL:     cfg.ZarinHub.BaseURL,
		Username:    cfg.ZarinHub.Username,
		Password:    cfg.ZarinHub.Password,
		StaticToken: cfg.ZarinHub.StaticToken,
		Timeout:     cfg.ZarinHub.Timeout,
		TokenSkew:   cfg.ZarinHub.TokenSkew,
	}, logger)

	service := inquiry.NewService(provider, store, logger,
		inquiry.WithCardHashSecret(cfg.Audit.CardHashSecret),
	)

	router := httpapi.NewRouter(service, logger, httpapi.RouterConfig{
		AllowedOrigins:    cfg.HTTP.AllowedOrigins,
		TrustProxyHeaders: os.Getenv("TRUST_PROXY_HEADERS") == "true",
	}, store.DB().PingContext)

	server := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,

		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server listening",
			slog.String("addr", cfg.HTTP.Addr),
			slog.Any("cors_allowed_origins", cfg.HTTP.AllowedOrigins),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received", slog.Duration("grace_period", cfg.HTTP.ShutdownTimeout))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("server stopped cleanly")
	return nil
}
