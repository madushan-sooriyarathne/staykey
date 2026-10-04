// Command api runs the StayKey HTTP API.
package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"staykey.direct/api/internal/auth"
	"staykey.direct/api/internal/config"
	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/secret"
	"staykey.direct/api/internal/server"
	"staykey.direct/api/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := newLogger(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	box, err := secret.New(cfg.DataKey)
	if err != nil {
		return err
	}
	db, err := store.NewPostgres(ctx, cfg.DatabaseURL, store.WithSecrets(box))
	if err != nil {
		return err
	}
	defer db.Close()

	fileStore, media, err := newFileStore(cfg)
	if err != nil {
		return err
	}

	authService, err := auth.New(db, auth.LogSender{Logger: log}, auth.Config{
		Secret: []byte(cfg.AuthSecret),
		// Development builds show the code on screen, so a phone can sign in without SMS.
		ExposeDevCodes: !cfg.IsProduction(),
		Logger:         log,
	})
	if err != nil {
		return err
	}

	srv, err := server.New(db, authService, fileStore, server.Options{
		BookingDomain:   cfg.BookingDomain,
		BookingScheme:   cfg.BookingScheme,
		AllowAllOrigins: !cfg.IsProduction(),
		ClientIPHeader:  cfg.ClientIPHeader,
		PublicURL:       cfg.PublicURL,
		Media:           media,
		Logger:          log,
	})
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", httpServer.Addr, "env", cfg.Env)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

// newFileStore returns R2 in production and a folder served by the API in development.
func newFileStore(cfg config.Config) (files.Store, http.Handler, error) {
	if cfg.Storage == "r2" {
		r2, err := files.NewR2(files.R2Config(cfg.R2))
		return r2, nil, err
	}
	// The upload signing key is derived from the data key rather than reusing it.
	mac := hmac.New(sha256.New, cfg.DataKey)
	mac.Write([]byte("staykey local uploads v1"))
	local, err := files.NewLocal(cfg.MediaDir, mac.Sum(nil))
	if err != nil {
		return nil, nil, err
	}
	return local, local.Handler(), nil
}

func newLogger(cfg config.Config) *slog.Logger {
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}
