// Command api runs the StayKey HTTP API.
package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/auth"
	"staykey.direct/api/internal/billing"
	"staykey.direct/api/internal/cards"
	"staykey.direct/api/internal/config"
	"staykey.direct/api/internal/email"
	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/ical"
	"staykey.direct/api/internal/jobs"
	"staykey.direct/api/internal/notify"
	"staykey.direct/api/internal/observe"
	"staykey.direct/api/internal/secret"
	"staykey.direct/api/internal/server"
	"staykey.direct/api/internal/sms"
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
	reporting, err := observe.Init(observe.Config{DSN: cfg.Observe.SentryDSN, Environment: cfg.Env, Release: cfg.Observe.Release})
	if err != nil {
		return fmt.Errorf("sentry: %w", err)
	}
	log.Info("error reporting", "sentry", reporting, "release", cfg.Observe.Release)
	defer observe.Flush(2 * time.Second)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	box, err := secret.New(cfg.DataKey)
	if err != nil {
		return err
	}
	links := store.Links{
		BookingPage: func(slug string) string { return cfg.BookingScheme + "://" + slug + "." + cfg.BookingDomain },
		VerifyEmail: cfg.VerifyEmailURL,
	}
	collectMode := ""
	if k := cfg.Cards.CollectSecretKey; k != "" {
		collectMode = billing.Merchant{SecretKey: k}.Mode()
	}
	db, err := store.NewPostgres(ctx, cfg.DatabaseURL, store.WithSecrets(box), store.WithLinks(links),
		store.WithCards(store.CardConfig{
			CollectMode: collectMode, PayoutsEnabled: cfg.Cards.PayoutsEnabled, PayoutFeeBPS: cfg.Cards.PayoutFeeBPS,
		}))
	if err != nil {
		return err
	}
	defer db.Close()
	if cfg.IsProduction() {
		// The pool opens lazily so the API can start while the database is briefly away; only a
		// role that definitely skips row-level security stops it.
		bypass, err := db.BypassesRowSecurity(ctx)
		switch {
		case err != nil:
			log.Warn("couldn't check the database role", "error", err)
		case bypass:
			return errors.New("DATABASE_URL connects as a superuser or BYPASSRLS role, which skips row-level security; connect as an ordinary role (scripts/db-app-role.sql)")
		}
	}

	fileStore, media, err := newFileStore(cfg)
	if err != nil {
		return err
	}

	var whatsApp notify.WhatsApp = notify.LogWhatsApp{Logger: log}
	if cfg.WhatsAppProvider == "cloud" {
		w := cfg.WhatsApp
		whatsApp = notify.CloudWhatsApp{Token: w.Token, PhoneNumberID: w.PhoneNumberID, Template: w.Template}
	}
	// Development logs each email's link, so confirmation and stay links open locally.
	var mailer email.Sender = email.LogSender{Logger: log, ShowLinks: !cfg.IsProduction()}
	if cfg.EmailProvider == "ses" {
		e := cfg.SES
		mailer, err = email.NewSES(email.SESConfig{
			Region: e.Region, AccessKeyID: e.AccessKeyID, SecretAccessKey: e.SecretAccessKey,
			From: e.From, ConfigurationSet: e.ConfigurationSet,
		})
		if err != nil {
			return err
		}
	}
	cardService := &cards.Service{
		Store: db, CollectKey: cfg.Cards.CollectSecretKey, Production: cfg.IsProduction(), BaseURL: cfg.PaymentsLK.APIURL,
		StayLink: func(slug string, bookingID uuid.UUID, token string) string {
			return links.BookingPage(slug) + "/stay/" + bookingID.String() + "?t=" + token
		},
		Logger: log,
	}
	alerts := jobs.Alerts{Push: notify.Expo{AccessToken: cfg.ExpoAccessToken}, WhatsApp: whatsApp, Email: mailer, Logger: log}
	beats := &observe.Heartbeats{URLs: cfg.Observe.Heartbeats, Logger: log}
	jobClient, err := jobs.Start(ctx, db.Pool(), db, fileStore, alerts, ical.NewFetcher(cfg.ICalAllowPrivate), cardService, beats, log)
	if err != nil {
		return err
	}

	var lk auth.SMSSender = auth.LogSender{Logger: log}
	if cfg.SMSProvider == "ozone" {
		lk = sms.Ozone{UserID: cfg.Ozone.UserID, APIKey: cfg.Ozone.APIKey, SenderID: cfg.Ozone.SenderID}
	}
	authService, err := auth.New(db, sms.Router{LK: lk}, auth.Config{
		Secret: []byte(cfg.AuthSecret),
		// Development builds show the code on screen, so a phone can sign in without SMS.
		ExposeDevCodes: !cfg.IsProduction(),
		ReviewPhone:    cfg.ReviewPhone,
		ReviewCode:     cfg.ReviewCode,
		Logger:         log,
	})
	if err != nil {
		return err
	}

	opts := server.Options{
		BookingDomain:   cfg.BookingDomain,
		BookingScheme:   cfg.BookingScheme,
		AllowAllOrigins: !cfg.IsProduction(),
		ClientIPHeader:  cfg.ClientIPHeader,
		PublicURL:       cfg.PublicURL,
		InviteURL:       cfg.InviteURL,
		Media:           media,
		Logger:          log,
		ForwardSecret:   cfg.ForwardSecret,
		OpsSecret:       cfg.Observe.OpsSecret,
		BillingURL:      cfg.BillingURL,
		Cards:           cardService,
	}
	if rc := cfg.RevenueCat; rc.SecretKey != "" {
		client := billing.RevenueCat{SecretKey: rc.SecretKey, ExternalKey: rc.ExternalKey}
		opts.Subscribers, opts.ExternalPurchases, opts.RevenueCatWebhookAuth = client, client, rc.WebhookAuth
	}
	if cfg.SES.SNSTopicARN != "" {
		opts.SNS = &email.SNS{Region: cfg.SES.Region, TopicARN: cfg.SES.SNSTopicARN}
	}
	if lk := cfg.PaymentsLK; lk.SecretKey != "" {
		opts.WebPayments = billing.PaymentsLK{SecretKey: lk.SecretKey, WebhookSecret: lk.WebhookSecret, BaseURL: lk.APIURL}
	}
	srv, err := server.New(db, authService, fileStore, opts)
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
	return errors.Join(httpServer.Shutdown(shutdownCtx), jobClient.Stop(shutdownCtx))
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

// newLogger writes JSON in production, so the log service can search its fields, and text in
// development. Every line written with a request's context carries the request id.
func newLogger(cfg config.Config) *slog.Logger {
	if cfg.IsProduction() {
		return slog.New(observe.LogHandler(slog.NewJSONHandler(os.Stdout, nil)))
	}
	return slog.New(observe.LogHandler(slog.NewTextHandler(os.Stdout, nil)))
}
