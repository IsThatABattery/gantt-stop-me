// Command runner builds and runs workflow executions' containers for the API. It must run as a container
// on the Docker daemon it drives, since it joins each execution's network.
package main

import (
	"cmp"
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"gantt-stop-me/api/internal/runner"
)

func main() {
	token := os.Getenv("RUNNER_TOKEN")
	if token == "" {
		log.Fatal("RUNNER_TOKEN is required. Generate one: openssl rand -base64 32")
	}
	// Docker sets a container's hostname to its short id, which is how the runner finds its own container.
	self, err := os.Hostname()
	if err != nil {
		log.Fatal("hostname: ", err)
	}
	s := &runner.Server{Token: token, Prefix: cmp.Or(os.Getenv("RUN_PREFIX"), "gantt"), Work: "/work", Self: self}
	if err := s.DeleteAll(context.Background()); err != nil {
		log.Fatal("remove what an earlier runner left: ", err)
	}
	srv := &http.Server{
		Addr:              ":8081",
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("runner listening on :8081, prefix %s", s.Prefix)
	log.Fatal(srv.ListenAndServe())
}
