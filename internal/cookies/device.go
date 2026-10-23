package cookies

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// DeviceSource stands in for a cookie jar when a proxy holds the account
// cookies. It supplies only the web-player device id (`sp_t`), which is not a
// credential.
type DeviceSource struct {
	deviceID string
}

func NewDeviceSource() (DeviceSource, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return DeviceSource{}, err
	}
	return DeviceSource{deviceID: hex.EncodeToString(raw)}, nil
}

func (s DeviceSource) Cookies(ctx context.Context) ([]*http.Cookie, error) {
	_ = ctx
	return []*http.Cookie{{Name: "sp_t", Value: s.deviceID, Domain: ".spotify.com", Path: "/"}}, nil
}
