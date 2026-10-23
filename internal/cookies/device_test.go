package cookies

import (
	"context"
	"regexp"
	"testing"
)

func TestDeviceSourceSuppliesOnlyDeviceID(t *testing.T) {
	source, err := NewDeviceSource()
	if err != nil {
		t.Fatalf("new source: %v", err)
	}
	cookies, err := source.Cookies(context.Background())
	if err != nil {
		t.Fatalf("cookies: %v", err)
	}
	if len(cookies) != 1 || cookies[0].Name != "sp_t" || !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(cookies[0].Value) {
		t.Fatalf("unexpected cookies: %#v", cookies)
	}
	again, _ := source.Cookies(context.Background())
	if again[0].Value != cookies[0].Value {
		t.Fatalf("device id must be stable within a process")
	}
	other, _ := NewDeviceSource()
	if otherCookies, _ := other.Cookies(context.Background()); otherCookies[0].Value == cookies[0].Value {
		t.Fatalf("device ids must differ between processes")
	}
}
