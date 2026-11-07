package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"gantt-stop-me/api/internal/httpx"
)

// modelsClient fetches user-supplied URLs from inside our network. It never follows a redirect,
// so the URL the user entered is the only one the API calls.
var modelsClient = &http.Client{
	Timeout:       10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

var (
	errKeyRejected = errors.New("provider rejected the key")
	errUnreachable = errors.New("provider unreachable")
)

// listModels returns the model ids an OpenAI-compatible endpoint offers to key, from GET {baseURL}/models.
func listModels(ctx context.Context, baseURL, key string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, errUnreachable
	}
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := modelsClient.Do(req)
	if err != nil {
		return nil, errUnreachable
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, errKeyRejected
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if res.StatusCode/100 != 2 || json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body) != nil {
		return nil, errUnreachable
	}
	ids := make([]string, len(body.Data))
	for i, m := range body.Data {
		ids[i] = m.ID
	}
	return ids, nil
}

// checkModels fetches the listing at baseURL with key and checks every wanted model is in it.
// On failure it answers 400 and returns false; the upstream's status and body never reach the client.
func checkModels(w http.ResponseWriter, r *http.Request, baseURL, key string, wanted []string) ([]string, bool) {
	available, err := listModels(r.Context(), baseURL, key)
	if errors.Is(err, errKeyRejected) {
		httpx.Error(w, http.StatusBadRequest, "invalid_api_key", "The provider rejected this API key.")
		return nil, false
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "provider_unreachable", "Could not list models at this base URL.")
		return nil, false
	}
	for _, m := range wanted {
		if !slices.Contains(available, m) {
			httpx.Error(w, http.StatusBadRequest, "unknown_model", fmt.Sprintf("The provider does not offer model %q.", m))
			return nil, false
		}
	}
	return available, true
}
