package content

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// httpTimeoutSeconds caps a single quota-API call. Kept generous because the
// upstream OAuth-usage endpoint occasionally takes a few seconds to respond
// during peak hours, and timing out turns into a fake "API unavailable"
// state in the cache.
const httpTimeoutSeconds = 15

// claudeAPIProxy holds the proxy URL applied only to api.anthropic.com requests.
// Empty (default) → no proxy. Precedence resolution (CLI > env > YAML) happens
// in (*config.Config).ResolveClaudeAPIProxy and is passed in via SetClaudeAPIProxy.
var (
	claudeAPIProxy   string
	claudeAPIProxyMu sync.RWMutex
)

// SetClaudeAPIProxy stores the already-resolved proxy URL used for outbound
// requests to api.anthropic.com. An empty string disables the proxy.
// Thread-safe. Callers should pass the value from
// (*config.Config).ResolveClaudeAPIProxy so CLI / env / YAML precedence stays
// in one place.
func SetClaudeAPIProxy(proxyURL string) {
	claudeAPIProxyMu.Lock()
	defer claudeAPIProxyMu.Unlock()
	claudeAPIProxy = strings.TrimSpace(proxyURL)
}

// getClaudeAPIProxy returns the stored proxy URL for Claude API requests.
// Returns an empty string when no proxy is configured.
func getClaudeAPIProxy() string {
	claudeAPIProxyMu.RLock()
	defer claudeAPIProxyMu.RUnlock()
	return claudeAPIProxy
}

// newClaudeHTTPClient returns an HTTP client for Claude OAuth API requests.
// When a proxy is configured it routes through that proxy; otherwise it uses
// a direct connection. It deliberately does NOT honor HTTP_PROXY/HTTPS_PROXY
// so unrelated environment proxies cannot leak into Claude API traffic.
//
// Supported schemes:
//   - http://  / https:// — standard HTTP CONNECT proxy. Basic-auth credentials
//     embedded in the URL (user:pass@host) are sent automatically by
//     net/http via the Proxy-Authorization header.
//   - socks5:// / socks5h:// — SOCKS5 proxy via golang.org/x/net/proxy.
//     SOCKS5 username/password auth is read from the URL by proxy.FromURL.
//
// Unknown or unparseable schemes silently fall through to a direct connection
// rather than failing — a typo in the YAML must never break the statusline.
func newClaudeHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{Proxy: nil}
	if raw := getClaudeAPIProxy(); raw != "" {
		applyProxyToTransport(transport, raw)
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

// applyProxyToTransport mutates transport so requests route through the proxy
// described by rawURL. Returns silently on any parse / scheme error so that a
// malformed YAML value never escalates into a startup failure.
func applyProxyToTransport(transport *http.Transport, rawURL string) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		// http.ProxyURL takes care of Basic-auth credentials in user info.
		transport.Proxy = http.ProxyURL(parsed)
	case "socks5", "socks5h":
		// proxy.FromURL reads SOCKS5 user/password from the URL's user info.
		dialer, err := proxy.FromURL(parsed, proxy.Direct)
		if err != nil {
			return
		}
		// Only ContextDialer integrates cleanly with http.Transport — every
		// dialer in x/net/proxy implements it, but guard anyway for forward
		// compatibility.
		if cd, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = cd.DialContext
		}
	}
}

// parseRetryAfterHeader parses Retry-After header value. Shared by the
// Anthropic OAuth and GLM monitor fetchers because both upstreams use the
// same RFC 7231 semantics (seconds OR HTTP-date). Returns 0 when the header
// is empty or unparseable so the caller falls back to exponential backoff.
//
// Can be either seconds or HTTP date format.
func parseRetryAfterHeader(value string) int {
	if value == "" {
		return 0
	}

	// Try parsing as seconds
	if sec, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && sec > 0 {
		return sec
	}

	// Try parsing as HTTP date
	if t, err := time.Parse(time.RFC1123, value); err == nil {
		sec := int(time.Until(t).Seconds())
		if sec > 0 {
			return sec
		}
	}

	return 0
}
