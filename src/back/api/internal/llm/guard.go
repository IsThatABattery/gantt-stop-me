package llm

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// CallTimeout bounds one decisions or chat completions call, which a model may think about for a while.
const CallTimeout = 120 * time.Second

// HTTPClient returns a client for user-supplied provider URLs, called from inside our network, that gives up after
// timeout. It never follows a redirect, so the URL the user entered is the only one the API calls, and it dials only
// addresses dialGuard allows: loopback and private ones only when allowPrivate (ALLOW_PRIVATE_PROVIDER_URLS).
func HTTPClient(allowPrivate bool, timeout time.Duration) *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil // through a proxy, the guard would check the proxy's address instead of the provider's
	t.DialContext = (&net.Dialer{Timeout: 10 * time.Second, Control: dialGuard(allowPrivate)}).DialContext
	return &http.Client{
		Transport:     t,
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

var errBlocked = errors.New("provider address not allowed")

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
