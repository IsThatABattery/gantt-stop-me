package app

import "net/http"

// health answers "ok" once the API is serving.
func health(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}
