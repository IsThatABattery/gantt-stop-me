// Package httpx holds the JSON, error and origin helpers every handler uses.
package httpx

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
)

// JSON writes v as the response body with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

// ErrorBody is the inside of the API's one error shape.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error writes {"error": {"code": code, "message": message}}.
func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, map[string]ErrorBody{"error": {Code: code, Message: message}})
}

// Internal logs err and answers 500 without leaking it to the client.
func Internal(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	Error(w, http.StatusInternalServerError, "internal", "Something went wrong.")
}

// Decode reads a JSON body of at most 1 MiB into v. On failure it answers 400 and returns false.
func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "Request body must be valid JSON.")
		return false
	}
	return true
}

// CheckOrigin refuses state-changing requests whose Origin header is not publicURL's origin,
// so another site cannot act with the user's session cookie.
func CheckOrigin(publicURL string, next http.Handler) http.Handler {
	u, err := url.Parse(publicURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		panic("httpx: PUBLIC_URL must be an absolute URL, got " + publicURL)
	}
	origin := u.Scheme + "://" + u.Host
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if r.Header.Get("Origin") != origin {
				Error(w, http.StatusForbidden, "bad_origin", "Cross-origin request refused.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
