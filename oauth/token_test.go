package oauth

import (
	"testing"
	"time"
)

func TestTokenAccessExpiry(t *testing.T) {
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	cases := []struct {
		name string
		tok  *Token
		want bool
	}{
		{name: "12 seconds", tok: &Token{Expiry: now.Add(12*time.Second + accessExpiryDelta)}, want: false},
		{name: "10 seconds", tok: &Token{Expiry: now.Add(expiryDelta + accessExpiryDelta)}, want: false},
		{name: "10 seconds-1ns", tok: &Token{Expiry: now.Add(expiryDelta + accessExpiryDelta - 1*time.Nanosecond)}, want: true},
		{name: "-1 hour", tok: &Token{Expiry: now.Add(-1 * time.Hour)}, want: true},
	}
	for _, tc := range cases {
		if got, want := tc.tok.expired(), tc.want; got != want {
			t.Errorf("expired (%q) = %v; want %v", tc.name, got, want)
		}
	}
}

func TestTokenRefreshExpiry(t *testing.T) {
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	cases := []struct {
		name string
		tok  *Token
		want bool
	}{
		{name: "12 seconds", tok: &Token{Expiry: now.Add(12 * time.Second)}, want: false},
		{name: "10 seconds", tok: &Token{Expiry: now.Add(expiryDelta)}, want: false},
		{name: "10 seconds-1ns", tok: &Token{Expiry: now.Add(expiryDelta - 1*time.Nanosecond)}, want: true},
		{name: "-1 hour", tok: &Token{Expiry: now.Add(-1 * time.Hour)}, want: true},
	}
	for _, tc := range cases {
		if got, want := tc.tok.refreshExpired(), tc.want; got != want {
			t.Errorf("expired (%q) = %v; want %v", tc.name, got, want)
		}
	}
}
