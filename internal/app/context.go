package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/steipete/spogo/internal/config"
	"github.com/steipete/spogo/internal/cookies"
	"github.com/steipete/spogo/internal/output"
	"github.com/steipete/spogo/internal/spotify"
)

type Settings struct {
	ConfigPath string
	Profile    string
	Timeout    time.Duration
	Market     string
	Language   string
	Device     string
	Engine     string
	Format     output.Format
	NoColor    bool
	Quiet      bool
	Verbose    bool
	Debug      bool
}

type Context struct {
	Settings   Settings
	Config     *config.Config
	ConfigPath string
	Profile    config.Profile
	ProfileKey string
	Output     *output.Writer

	// proxy is set by SPOGO_PROXY_URL + SPOGO_PROXY_TOKEN (environment only, so
	// the capability token never appears in argv). A proxy holds the account
	// cookies and access tokens; spogo then needs neither.
	proxy         *spotify.ProxyTransport
	spotifyClient spotify.API
}

func NewContext(settings Settings) (*Context, error) {
	configPath := settings.ConfigPath
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	if configPath == "" {
		configPath, err = config.DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	profileKey := settings.Profile
	if profileKey == "" {
		profileKey = cfg.DefaultProfile
	}
	if profileKey == "" {
		profileKey = config.DefaultProfile
	}
	profile := cfg.Profile(profileKey)
	if settings.Market != "" {
		profile.Market = settings.Market
	}
	if settings.Language != "" {
		profile.Language = settings.Language
	}
	if settings.Device != "" {
		profile.Device = settings.Device
	}
	if settings.Engine != "" {
		profile.Engine = settings.Engine
	}
	format := settings.Format
	if format == "" {
		format = output.FormatHuman
	}
	colorEnabled := isColorEnabled(format, settings.NoColor)
	writer, err := output.New(output.Options{
		Format: format,
		Color:  colorEnabled,
		Quiet:  settings.Quiet,
	})
	if err != nil {
		return nil, err
	}
	var proxy *spotify.ProxyTransport
	if rawURL := strings.TrimSpace(os.Getenv("SPOGO_PROXY_URL")); rawURL != "" {
		proxy, err = spotify.NewProxyTransport(rawURL, os.Getenv("SPOGO_PROXY_TOKEN"))
		if err != nil {
			return nil, err
		}
	}
	return &Context{
		Settings:   settings,
		Config:     cfg,
		ConfigPath: configPath,
		Profile:    profile,
		ProfileKey: profileKey,
		Output:     writer,
		proxy:      proxy,
	}, nil
}

func (c *Context) Spotify() (spotify.API, error) {
	if c == nil {
		return nil, errors.New("nil context")
	}
	if c.spotifyClient != nil {
		return c.spotifyClient, nil
	}
	source, err := c.cookieSource()
	if err != nil {
		return nil, err
	}
	engine := strings.ToLower(strings.TrimSpace(c.Profile.Engine))
	if engine == "" {
		engine = "connect"
	}
	switch engine {
	case "connect":
		client, err := spotify.NewConnectClient(c.connectOptions(source))
		if err != nil {
			return nil, err
		}
		c.spotifyClient = client
		return client, nil
	case "web":
		webClient, err := spotify.NewClient(c.webOptions(source))
		if err != nil {
			return nil, err
		}
		client := spotify.API(webClient)
		if connectClient, connectErr := spotify.NewConnectClient(c.connectOptions(source)); connectErr == nil {
			client = spotify.NewPlaybackFallbackClient(webClient, connectClient)
		}
		c.spotifyClient = client
		return client, nil
	case "auto":
		webClient, err := spotify.NewClient(c.webOptions(source))
		if err != nil {
			return nil, err
		}
		client := spotify.API(webClient)
		if connectClient, connectErr := spotify.NewConnectClient(c.connectOptions(source)); connectErr == nil {
			client = spotify.NewAutoClient(connectClient, webClient)
		}
		c.spotifyClient = client
		return client, nil
	case "applescript":
		var fallback spotify.API
		if webClient, webErr := spotify.NewClient(c.webOptions(source)); webErr == nil {
			fallback = webClient
		}
		client, err := spotify.NewAppleScriptClient(spotify.AppleScriptOptions{
			Fallback: fallback,
		})
		if err != nil {
			return nil, err
		}
		c.spotifyClient = client
		return client, nil
	default:
		return nil, fmt.Errorf("unknown engine %q (use auto, web, connect, or applescript)", engine)
	}
}

func (c *Context) connectOptions(source cookies.Source) spotify.ConnectOptions {
	return spotify.ConnectOptions{
		Source:    source,
		Market:    c.Profile.Market,
		Language:  c.Profile.Language,
		Device:    c.Profile.Device,
		Timeout:   c.Settings.Timeout,
		Transport: c.transport(),
	}
}

func (c *Context) webOptions(source cookies.Source) spotify.Options {
	provider := spotify.CookieTokenProvider{Source: source}
	if c.proxy != nil {
		provider.Client = &http.Client{Transport: c.proxy, Timeout: c.EnsureTimeout()}
	}
	return spotify.Options{
		TokenProvider: provider,
		Market:        c.Profile.Market,
		Language:      c.Profile.Language,
		Device:        c.Profile.Device,
		Timeout:       c.Settings.Timeout,
		Transport:     c.transport(),
	}
}

func (c *Context) transport() http.RoundTripper {
	if c.proxy == nil {
		return nil
	}
	return c.proxy
}

// Proxied reports whether a proxy holds the Spotify credentials.
func (c *Context) Proxied() bool {
	return c != nil && c.proxy != nil
}

// ProxyToken fetches a web-player token through the proxy. The proxy withholds
// the real access token; the call proves the proxy holds a working session.
func (c *Context) ProxyToken(ctx context.Context) (spotify.Token, error) {
	source, err := c.cookieSource()
	if err != nil {
		return spotify.Token{}, err
	}
	return c.webOptions(source).TokenProvider.Token(ctx)
}

func (c *Context) SetSpotify(client spotify.API) {
	if c == nil {
		return
	}
	c.spotifyClient = client
}

func (c *Context) cookieSource() (cookies.Source, error) {
	if c.proxy != nil {
		return cookies.NewDeviceSource()
	}
	if c.Profile.CookiePath != "" {
		return cookies.FileSource{Path: c.Profile.CookiePath}, nil
	}
	// Prefer the resolved cookie cache path (created by `spogo auth import` and
	// used by `spogo auth status`) over browser extraction. This keeps runtime
	// behavior consistent with auth commands and avoids needing direct browser
	// cookie access in headless/CI environments.
	if resolved := c.ResolveCookiePath(); strings.TrimSpace(resolved) != "" {
		if _, err := os.Stat(resolved); err == nil {
			return cookies.FileSource{Path: resolved}, nil
		}
	}
	browser := c.Profile.Browser
	if strings.TrimSpace(browser) == "" {
		browser = "chrome"
	}
	return cookies.BrowserSource{
		Browser: browser,
		Profile: c.Profile.BrowserProfile,
		Domain:  "spotify.com",
	}, nil
}

func (c *Context) SaveProfile(profile config.Profile) error {
	if c == nil {
		return errors.New("nil context")
	}
	cfg := c.Config
	cfg.SetProfile(c.ProfileKey, profile)
	cfg.DefaultProfile = c.ProfileKey
	if err := config.Save(c.ConfigPath, cfg); err != nil {
		return err
	}
	c.Profile = profile
	return nil
}

func isColorEnabled(format output.Format, noColor bool) bool {
	if format != output.FormatHuman {
		return false
	}
	if noColor {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := strings.ToLower(os.Getenv("TERM"))
	if term == "dumb" {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd())
}

func (c *Context) ResolveCookiePath() string {
	return config.CookiePath(c.ConfigPath, c.ProfileKey)
}

func (c *Context) EnsureTimeout() time.Duration {
	if c.Settings.Timeout > 0 {
		return c.Settings.Timeout
	}
	return 10 * time.Second
}

func (c *Context) ValidateProfile() error {
	if c.Profile.Market != "" && len(c.Profile.Market) != 2 {
		return fmt.Errorf("market must be 2-letter country code")
	}
	return nil
}
