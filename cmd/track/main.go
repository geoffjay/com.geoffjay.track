// Command track runs the rowing-miles tracker web service.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"com.geoffjay.track/internal/auth"
	"com.geoffjay.track/internal/config"
	"com.geoffjay.track/internal/db"
	"com.geoffjay.track/internal/track"
	"com.geoffjay.track/internal/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.IsProduction() {
		os.Setenv("GIN_MODE", "release")
	}

	dbh, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer dbh.Close()

	// Seed the two predefined users. Passwords come from the environment so
	// they never live in source; geoff/misty defaults cover local dev.
	authStore := auth.NewStore(dbh)
	seed := []auth.User{
		{Username: "geoff", PasswordHash: envOr("TRACK_GEOFF_PW", "geoff-row")},
		{Username: "misty", PasswordHash: envOr("TRACK_MISTY_PW", "misty-row")},
	}
	if err := authStore.EnsureUsers(context.Background(), seed, cfg.PasswordCost); err != nil {
		return err
	}

	tracker := track.NewStore(dbh)
	srv := web.New(cfg, authStore, tracker)

	httpSrv := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.Port),
		Handler: srv.Router(),
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	done := make(chan error, 1)
	go func() {
		log.Printf("track listening on :%d (env=%s db=%s)", cfg.Port, cfg.Env, cfg.DBPath)
		done <- httpSrv.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-done:
		return err
	case <-sig:
		ctx, cancel := context.WithTimeout(context.Background(), config.GracefulShutdownTimeout)
		defer cancel()
		return httpSrv.Shutdown(ctx)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}