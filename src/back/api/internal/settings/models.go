package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"syscall"
	"time"

	"gantt-stop-me/api/internal/httpx"
)

// modelsClient fetches user-supplied URLs from inside our network. It never follows a redirect,
// so the URL the user entered is the only one the API calls, and it dials only addresses dialGuard allows.
func modelsClient(allowPrivate bool) *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil // through a proxy, the guard would check the proxy's address instead of the provider's
	t.DialContext = (&net.Dialer{Timeout: 10 * time.Second, Control: dialGuard(allowPrivate)}).DialContext
	return &http.Client{
		Transport:     t,
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// dialGuard refuses link-local addresses, such as cloud metadata at 169.254.169.254, and loopback and private
// ones unless allowPrivate. It checks the address being dialled, after DNS, so a rebinding name cannot pass it.
func dialGuard(allowPrivate bool) func(network, address string, c syscall.RawConn) error {
	return func(_, address string, _ syscall.RawConn) error {
		ap, err := netip.ParseAddrPort(address)
		if err != nil {
			return err
		}
		a := ap.Addr().Unmap()
		if a.IsLinkLocalUnicast() || a.IsMulticast() || a.IsUnspecified() ||
			!allowPrivate && (a.IsLoopback() || a.IsPrivate() || cgnat.Contains(a)) {
			return errBlocked
		}
		return nil
	}
}

// maxModels is the most models one provider can have chosen.
const maxModels = 100

var (
	errKeyRejected = errors.New("provider rejected the key")
	errUnreachable = errors.New("provider unreachable")
	errBlocked     = errors.New("provider address not allowed")
)

// listModels returns the model ids an OpenAI-compatible endpoint offers to key, from GET {baseURL}/models.
func (s *Settings) listModels(ctx context.Context, baseURL, key string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, errUnreachable
	}
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := s.models.Do(req)
	if err != nil {
		return nil, errUnreachable
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, errKeyRejected
	}
	var body struct {
		Data *[]struct { // a pointer, so a body without a data list is not an empty listing
			ID string `json:"id"`
		} `json:"data"`
	}
	if res.StatusCode/100 != 2 || json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body) != nil || body.Data == nil {
		return nil, errUnreachable
	}
	ids := make([]string, len(*body.Data))
	for i, m := range *body.Data {
		ids[i] = m.ID
	}
	return ids, nil
}

// checkModels fetches the listing at baseURL with key and checks every wanted model is in it.
// On failure it answers 400 and returns false; the upstream's status and body never reach the client.
func (s *Settings) checkModels(w http.ResponseWriter, r *http.Request, baseURL, key string, wanted []string) ([]string, bool) {
	if len(wanted) > maxModels { // also keeps the quadratic checks below cheap
		httpx.Error(w, http.StatusBadRequest, "too_many_models", fmt.Sprintf("Choose at most %d models.", maxModels))
		return nil, false
	}
	available, err := s.listModels(r.Context(), baseURL, key)
	if errors.Is(err, errKeyRejected) {
		httpx.Error(w, http.StatusBadRequest, "invalid_api_key", "The provider rejected this API key.")
		return nil, false
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "provider_unreachable", "Could not list models at this base URL.")
		return nil, false
	}
	for i, m := range wanted {
		if slices.Contains(wanted[:i], m) {
			httpx.Error(w, http.StatusBadRequest, "duplicate_model", fmt.Sprintf("Model %q is listed more than once.", m))
			return nil, false
		}
		if !slices.Contains(available, m) {
			httpx.Error(w, http.StatusBadRequest, "unknown_model", fmt.Sprintf("The provider does not offer model %q.", m))
			return nil, false
		}
	}
	return available, true
}
