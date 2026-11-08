// Package llm calls the automation owner's LLM providers, OpenAI-compatible endpoints at base URLs users enter:
// the Decisions API that routes tests, and the Chat Completions tool-calling loop that runs LLM checks.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

// maxAnswer is the most of one provider answer the client reads.
const maxAnswer = 4 << 20

// Client calls one provider with its key.
type Client struct {
	BaseURL string       // such as https://api.openai.com/v1, without a trailing slash, as settings stores it
	Key     string       // the provider's API key, sent as a bearer token
	HTTP    *http.Client // from HTTPClient with CallTimeout, so every call keeps the guard
}

// unreachable is a call that got no answer. Its text says only that, and it wraps the cause, so a caller can still
// tell a cancel from the provider being down.
type unreachable struct {
	path string
	err  error
}

func (u unreachable) Error() string {
	if t := (interface{ Timeout() bool })(nil); errors.As(u.err, &t) && t.Timeout() {
		return "llm: POST " + u.path + " timed out"
	}
	return "llm: POST " + u.path + " could not reach the provider"
}

func (u unreachable) Unwrap() error { return u.err }

// post sends in as JSON to {BaseURL}{path} and decodes a 2xx answer into out. Its errors never quote the answer,
// which the provider controls, or name the provider's address, since they end up in test results and issue comments.
func (c *Client) post(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("llm: POST %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		// Its text names the provider's address, which may be private, so only the log keeps it.
		log.Printf("llm: POST %s%s: %v", c.BaseURL, path, err)
		return unreachable{path, err}
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("llm: POST %s answered HTTP %d", path, res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxAnswer+1))
	if err != nil {
		return fmt.Errorf("llm: POST %s: %w", path, err)
	}
	if len(data) > maxAnswer {
		return fmt.Errorf("llm: POST %s answered over %d MiB", path, maxAnswer>>20)
	}
	if json.Unmarshal(data, out) != nil { // the decoder's message can quote the answer
		return fmt.Errorf("llm: POST %s answered malformed JSON", path)
	}
	return nil
}
