// Package llm calls the automation owner's LLM providers, OpenAI-compatible endpoints at base URLs users enter:
// the Decisions API that routes tests, and the Chat Completions tool-calling loop that runs LLM checks.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// post sends in as JSON to {BaseURL}{path} and decodes a 2xx answer into out. Its errors never quote the answer,
// which the provider controls, since they end up in test results anyone on the project reads.
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
		return fmt.Errorf("llm: POST %s: %w", path, err)
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
