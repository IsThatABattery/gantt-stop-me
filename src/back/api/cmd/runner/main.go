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
	prefix := cmp.Or(os.Getenv("RUN_PREFIX"), "gantt")
	if !runner.ValidPrefix(prefix) {
		log.Fatal("RUN_PREFIX must be 2 to 32 lowercase letters, digits or -, starting with a letter and ending in a letter or digit")
	}
	// Docker sets a container's hostname to its short id, which is how the runner finds its own container.
	self, err := os.Hostname()
	if err != nil {
		log.Fatal("hostname: ", err)
	}
	s := &runner.Server{Token: token, Prefix: prefix, Work: "/work", Self: self}
	// Before Listen, since it also leaves every execution network an earlier runner joined.
	if err := s.DeleteAll(context.Background()); err != nil {
		log.Fatal("remove what an earlier runner left: ", err)
	}
	ls, err := runner.Listen("8081")
	if err != nil {
		log.Fatal("listen: ", err)
	}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	errs := make(chan error)
	for _, l := range ls {
		log.Printf("runner listening on %s, prefix %s", l.Addr(), s.Prefix)
		go func() { errs <- srv.Serve(l) }()
	}
	log.Fatal(<-errs)
}
