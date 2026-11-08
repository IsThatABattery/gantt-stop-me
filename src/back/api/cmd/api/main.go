package main

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gantt-stop-me/api/internal/app"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/runner"
	"gantt-stop-me/api/internal/secret"
	"gantt-stop-me/api/internal/store"
	"gantt-stop-me/api/migrations"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	key, err := base64.StdEncoding.DecodeString(os.Getenv("ENCRYPTION_KEY"))
	if err != nil {
		log.Fatal("ENCRYPTION_KEY must be base64: ", err)
	}
	box, err := secret.New(key)
	if err != nil {
		log.Fatal("ENCRYPTION_KEY: ", err)
	}

	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("database: ", err)
	}
	if err := store.Migrate(ctx, db, migrations.FS); err != nil {
		log.Fatal(err)
	}

	maxWorkflows := 1
	if v := os.Getenv("MAX_CONCURRENT_WORKFLOWS"); v != "" {
		if maxWorkflows, err = strconv.Atoi(v); err != nil || maxWorkflows < 1 {
			log.Fatalf("MAX_CONCURRENT_WORKFLOWS must be a positive integer, not %q", v)
		}
	}
	runnerURL := strings.TrimSuffix(os.Getenv("RUNNER_URL"), "/")
	cfg := app.Config{
		PublicURL:                strings.TrimSuffix(os.Getenv("PUBLIC_URL"), "/"),
		Secrets:                  box,
		GitHub:                   &github.Client{API: "https://api.github.com", Web: "https://github.com", HTTP: http.DefaultClient},
		GitHubClientID:           os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret:       os.Getenv("GITHUB_CLIENT_SECRET"),
		AllowPrivateProviderURLs: os.Getenv("ALLOW_PRIVATE_PROVIDER_URLS") == "true",
		RunnerURL:                runnerURL,
		// No timeout for a whole request: a build can take as long as its workflow's timeout, which bounds each request.
		Runner:                 &runner.Client{URL: runnerURL, Token: os.Getenv("RUNNER_TOKEN"), HTTPClient: &http.Client{}},
		MaxConcurrentWorkflows: maxWorkflows,
	}
	api := app.New(db, cfg)
	if err := api.Automation.Recover(ctx); err != nil {
		log.Print("requeue the workflow executions that were running: ", err)
	}
	polled, worked := make(chan struct{}), make(chan struct{})
	go func() {
		api.Automation.Run(ctx)
		close(polled)
	}()
	go func() {
		api.Automation.RunWorker(ctx) // on shutdown, stops and removes the executions it runs and requeues them
		close(worked)
	}()
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           api,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Println("api listening on :8080")
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	stop() // a second signal ends the process at once
	log.Println("api shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Print("shutdown: ", err)
	}
	<-polled
	<-worked
	api.Automation.Wait() // webhook deliveries the server answered before it stopped
}
