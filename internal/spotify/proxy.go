package spotify

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const proxyTokenHeader = "x-mom-proxy-token"

// proxyProviders names the blind-proxy provider that holds credentials for each
// Spotify host spogo calls with an account token. The proxy authenticates the
// upstream request itself, so neither cookies nor access tokens exist here.
var proxyProviders = map[string]string{
	"api.spotify.com":          "spotify-api",
	"api-partner.spotify.com":  "spotify-partner",
	"spclient.wg.spotify.com":  "spotify-spclient",
	"image-upload.spotify.com": "spotify-upload",
}

const (
	tokenHost     = "open.spotify.com"
	tokenPath     = "/api/token"
	tokenProvider = "spotify-token"
)

// ProxyTransport sends credentialed Spotify requests through a blind proxy
// (`<base>/<provider><path>` plus a capability token header). Requests that
// carry no account credential go straight to Spotify.
type ProxyTransport struct {
	base  string
	token string
	next  http.RoundTripper
}

// NewProxyTransport validates the proxy base URL (for example
// http://127.0.0.1:3210/proxy) and capability token.
func NewProxyTransport(rawURL, token string) (*ProxyTransport, error) {
	base, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("invalid proxy url: %w", err)
	}
	if (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("invalid proxy url: expected http(s)://host[/path]")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("proxy token required")
	}
	return &ProxyTransport{
		base:  strings.TrimRight(base.String(), "/"),
		token: token,
		next:  http.DefaultTransport,
	}, nil
}

func (t *ProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	provider, err := proxyProvider(req.URL)
	if err != nil {
		return nil, err
	}
	if provider == "" {
		return t.next.RoundTrip(req)
	}
	target, err := url.Parse(t.base + "/" + provider + req.URL.EscapedPath())
	if err != nil {
		return nil, err
	}
	target.RawQuery = req.URL.RawQuery
	out := req.Clone(req.Context())
	out.URL = target
	out.Host = ""
	out.Header.Set(proxyTokenHeader, t.token)
	return t.next.RoundTrip(out)
}

func proxyProvider(u *url.URL) (string, error) {
	host := strings.ToLower(u.Hostname())
	switch {
	case host == tokenHost && u.Path == tokenPath:
		return tokenProvider, nil
	case host == tokenHost, host == "clienttoken.spotify.com":
		return "", nil
	}
	if provider, ok := proxyProviders[host]; ok {
		return provider, nil
	}
	if strings.HasSuffix(host, ".spotify.com") {
		return "", fmt.Errorf("%s is not available through the proxy", host)
	}
	return "", nil
}
