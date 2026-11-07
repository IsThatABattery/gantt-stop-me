package main

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gantt-stop-me/api/internal/app"
	"gantt-stop-me/api/internal/secret"
	"gantt-stop-me/api/internal/store"
	"gantt-stop-me/api/migrations"
)

func main() {
	ctx := context.Background()

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

	cfg := app.Config{
		PublicURL:                strings.TrimSuffix(os.Getenv("PUBLIC_URL"), "/"),
		Secrets:                  box,
		AllowPrivateProviderURLs: os.Getenv("ALLOW_PRIVATE_PROVIDER_URLS") == "true",
	}
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           app.New(db, cfg),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Println("api listening on :8080")
	log.Fatal(srv.ListenAndServe())
}
