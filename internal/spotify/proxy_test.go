package spotify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/steipete/spogo/internal/cookies"
)

func TestNewProxyTransportValidates(t *testing.T) {
	cases := map[string][2]string{
		"missing token":     {"http://127.0.0.1:3210/proxy", " "},
		"unsupported":       {"ftp://127.0.0.1/proxy", "tok"},
		"query not allowed": {"http://127.0.0.1/proxy?x=1", "tok"},
		"no host":           {"/proxy", "tok"},
	}
	for name, in := range cases {
		if _, err := NewProxyTransport(in[0], in[1]); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
	if _, err := NewProxyTransport("https://mom.internal:8443/proxy/", "tok"); err != nil {
		t.Fatalf("valid proxy: %v", err)
	}
}

func TestProxyTransportRouting(t *testing.T) {
	var seen *http.Request
	proxy, err := NewProxyTransport("http://127.0.0.1:3210/proxy/", "capability")
	if err != nil {
		t.Fatal(err)
	}
	proxy.next = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		seen = req
		return textResponse(http.StatusOK, ""), nil
	})

	rewritten := map[string]string{
		"https://open.spotify.com/api/token?reason=init&totp=1":       "http://127.0.0.1:3210/proxy/spotify-token/api/token?reason=init&totp=1",
		"https://api-partner.spotify.com/pathfinder/v2/query":         "http://127.0.0.1:3210/proxy/spotify-partner/pathfinder/v2/query",
		"https://spclient.wg.spotify.com/playlist/v2/playlist/p1?a=b": "http://127.0.0.1:3210/proxy/spotify-spclient/playlist/v2/playlist/p1?a=b",
		"https://image-upload.spotify.com/v4/playlist":                "http://127.0.0.1:3210/proxy/spotify-upload/v4/playlist",
		"https://api.spotify.com/v1/search?q=x%20y":                   "http://127.0.0.1:3210/proxy/spotify-api/v1/search?q=x%20y",
	}
	for in, want := range rewritten {
		req, _ := http.NewRequest(http.MethodGet, in, nil)
		req.Header.Set("Authorization", "Bearer placeholder")
		if _, err := proxy.RoundTrip(req); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if seen.URL.String() != want {
			t.Fatalf("%s: got %s want %s", in, seen.URL, want)
		}
		if seen.Header.Get("x-mom-proxy-token") != "capability" {
			t.Fatalf("%s: missing capability header", in)
		}
		if req.URL.Host == "127.0.0.1:3210" || req.Header.Get("x-mom-proxy-token") != "" {
			t.Fatalf("%s: caller request was mutated", in)
		}
	}

	direct := []string{
		"https://open.spotify.com/",
		"https://clienttoken.spotify.com/v1/clienttoken",
		"https://open.spotifycdn.com/cdn/build/web-player/web-player.js",
		"https://github.com/xyloflake/spot-secrets-go/blob/main/secrets/secretDict.json",
	}
	for _, in := range direct {
		req, _ := http.NewRequest(http.MethodGet, in, nil)
		if _, err := proxy.RoundTrip(req); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if seen.URL.String() != in || seen.Header.Get("x-mom-proxy-token") != "" {
			t.Fatalf("%s: expected direct request without capability, got %s", in, seen.URL)
		}
	}

	seen = nil
	req, _ := http.NewRequest(http.MethodPut, "https://gue1-spclient.spotify.com/connect-state/v1/devices/x", nil)
	if _, err := proxy.RoundTrip(req); err == nil || !strings.Contains(err.Error(), "not available through the proxy") {
		t.Fatalf("expected unavailable error, got %v", err)
	}
	if seen != nil {
		t.Fatalf("playback host must not be contacted")
	}
}

func TestConnectClientNeverSendsAccountCredentialsOutsideProxy(t *testing.T) {
	restore := SetTotpSecretFetcher(func(ctx context.Context) (int, []byte, error) {
		return 1, []byte{1, 2, 3, 4}, nil
	})
	t.Cleanup(restore)

	var direct []string
	var viaProxy []string
	network := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.Host == "proxy.test" || req.URL.Host == "proxy.test" {
			viaProxy = append(viaProxy, req.URL.Path)
			if req.Header.Get("x-mom-proxy-token") != "capability" {
				t.Fatalf("proxy request without capability: %s", req.URL)
			}
			if cookie := req.Header.Get("Cookie"); strings.Contains(cookie, "sp_dc") {
				t.Fatalf("account cookie sent to proxy: %s", cookie)
			}
			switch req.URL.Path {
			case "/proxy/spotify-token/api/token":
				return jsonResponse(http.StatusOK, tokenResponse{
					AccessToken:                    "placeholder",
					AccessTokenExpirationTimestamp: time.Now().Add(time.Hour).UnixMilli(),
					ClientID:                       "client",
				}), nil
			case "/proxy/spotify-upload/v4/playlist":
				return jsonResponse(http.StatusOK, map[string]any{"uploadToken": "upload-token"}), nil
			case "/proxy/spotify-spclient/playlist/v2/playlist/p1/register-image":
				return jsonResponse(http.StatusOK, map[string]any{"picture": "picture-ref"}), nil
			case "/proxy/spotify-spclient/playlist/v2/playlist/p1/changes":
				return jsonResponse(http.StatusOK, map[string]any{"revision": "r"}), nil
			case "/proxy/spotify-spclient/playlist/v2/playlist/p1":
				return jsonResponse(http.StatusOK, map[string]any{
					"ownerUsername": "me",
					"attributes":    map[string]any{"name": "Playlist", "picture": "picture-ref"},
				}), nil
			}
			return textResponse(http.StatusNotFound, req.URL.Path), nil
		}
		direct = append(direct, req.URL.Host+req.URL.Path)
		if req.Header.Get("x-mom-proxy-token") != "" || req.Header.Get("Authorization") != "" {
			t.Fatalf("credentialed direct request: %s", req.URL)
		}
		switch req.URL.Host {
		case "open.spotify.com":
			raw, _ := json.Marshal(map[string]any{"clientVersion": "1.2.3"})
			return textResponse(http.StatusOK, fmt.Sprintf(`<script id="appServerConfig" type="text/plain">%s</script>`, base64.StdEncoding.EncodeToString(raw))), nil
		case "clienttoken.spotify.com":
			return jsonResponse(http.StatusOK, map[string]any{"granted_token": map[string]any{"token": "client-token", "expires_in": 600}}), nil
		}
		t.Fatalf("unexpected direct request: %s", req.URL)
		return nil, nil
	})
	proxy, err := NewProxyTransport("http://proxy.test/proxy", "capability")
	if err != nil {
		t.Fatal(err)
	}
	proxy.next = network
	source, err := cookies.NewDeviceSource()
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewConnectClient(ConnectOptions{Source: source, Transport: proxy})
	if err != nil {
		t.Fatal(err)
	}

	item, err := client.UpdatePlaylist(context.Background(), "p1", PlaylistUpdate{ImageData: []byte{0xFF, 0xD8, 0xFF}})
	if err != nil {
		t.Fatalf("update playlist: %v", err)
	}
	if item.Picture != "picture-ref" {
		t.Fatalf("unexpected item: %#v", item)
	}
	for _, path := range []string{"/proxy/spotify-token/api/token", "/proxy/spotify-upload/v4/playlist", "/proxy/spotify-spclient/playlist/v2/playlist/p1/changes"} {
		if !containsString(viaProxy, path) {
			t.Fatalf("expected %s via proxy, got %v", path, viaProxy)
		}
	}
	for _, d := range direct {
		if !strings.HasPrefix(d, "open.spotify.com/") && !strings.HasPrefix(d, "clienttoken.spotify.com/") {
			t.Fatalf("account host reached directly: %s", d)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
