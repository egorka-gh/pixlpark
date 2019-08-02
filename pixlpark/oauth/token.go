// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package oauth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/egorka-gh/pixlpark/pixlpark/oauth/internal"
)

// expiryDelta determines how earlier a refreshtoken should be considered
// expired than its actual expiration time. It is used to avoid late
// expirations due to client-server time mismatches.
const expiryDelta = 10 * time.Second

// accessExpiryDelta determines how earlier a accesstoken should be considered expired.
// accesstoken should expire before refresh token so it must be > expiryDelta
//
const accessExpiryDelta = 60 * time.Second

// Token represents the credentials used to authorize
// the requests to access protected resources on the OAuth 2.0
// provider's backend.
//
// Most users of this package should not access fields of Token
// directly. They're exported mostly for use by related packages
// implementing derivative OAuth2 flows.
type Token struct {

	//Starting token /oauth/requesttoken
	RequestToken string `json:"RequestToken,omitempty"`

	// AccessToken is the token that authorizes and authenticates
	// the requests.
	AccessToken string `json:"AccessToken"`

	// TokenType is the type of token.
	// The Type method returns either this or "Bearer", the default.
	TokenType string `json:"token_type,omitempty"`

	// RefreshToken is a token that's used by the application
	// (as opposed to the user) to refresh the access token
	// if it expires.
	RefreshToken string `json:"RefreshToken,omitempty"`

	// Expiry is the expiration time of the access token and refresh token.
	// to separate expiration time of the access token used accessExpiryDelta
	//
	// If zero, TokenSource consider expired and not valid
	Expiry time.Time `json:"Expires,omitempty"`

	rawBody string
}

// Type returns t.TokenType if non-empty, else "Bearer".
func (t *Token) Type() string {
	if strings.EqualFold(t.TokenType, "bearer") {
		return "Bearer"
	}
	if strings.EqualFold(t.TokenType, "mac") {
		return "MAC"
	}
	if strings.EqualFold(t.TokenType, "basic") {
		return "Basic"
	}
	if t.TokenType != "" {
		return t.TokenType
	}
	return "Bearer"
}

// SetAuthHeader sets the Authorization header to r using the access
// token in t.
//
// This method is unnecessary when using Transport or an HTTP Client
// returned by this package.
func (t *Token) SetAuthHeader(r *http.Request) {
	r.Header.Set("Authorization", t.Type()+" "+t.AccessToken)
}

// SetAuthURLParametr adds the Authorization to r url query using the access
// token in t.
//
// This method is unnecessary when using Transport or an HTTP Client
// returned by this package.
func (t *Token) SetAuthURLParametr(r *http.Request) {
	q := r.URL.Query()
	q.Add("oauth_token", t.AccessToken)
	r.URL.RawQuery = q.Encode()
}

// timeNow is time.Now but pulled out as a variable for tests.
var timeNow = time.Now

// expired reports whether the access token is expired.
// t must be non-nil.
func (t *Token) expired() bool {
	if t.Expiry.IsZero() {
		return true
	}
	return t.Expiry.Round(0).Add(-accessExpiryDelta).Before(timeNow())
}

// refreshExpired reports whether the refresh token is expired.
// t must be non-nil.
func (t *Token) refreshExpired() bool {
	if t.Expiry.IsZero() {
		return true
	}
	return t.Expiry.Round(0).Add(-expiryDelta).Before(timeNow())
}

// Valid reports whether t is non-nil, has an AccessToken, and AccessToken is not expired.
func (t *Token) Valid() bool {
	return t != nil && t.AccessToken != "" && !t.expired()
}

// CanRefresh reports whether t is non-nil, has an RefreshToken, and RefreshToken is not expired.
func (t *Token) CanRefresh() bool {
	return t != nil && t.RefreshToken != "" && !t.refreshExpired()
}

// tokenFromInternal maps an *internal.Token struct into
// a *Token struct.
func tokenFromInternal(t *internal.Token) *Token {
	if t == nil {
		return nil
	}

	return &Token{
		RequestToken: t.RequestToken,
		AccessToken:  t.AccessToken,
		TokenType:    t.TokenType,
		RefreshToken: t.RefreshToken,
		Expiry:       t.Expiry,
		rawBody:      t.RawBody,
	}
}

// retrieveToken takes a *Config and uses that to retrieve an *internal.Token.
// This token is then mapped from *internal.Token into an *oauth2.Token which is returned along
// with an error..
func retrieveToken(ctx context.Context, c *Config, tokenURL string, v url.Values) (*Token, error) {
	c.Log("token url", tokenURL, "query", v.Encode())
	tk, err := internal.RetrieveToken(ctx, tokenURL, v)
	if err != nil {
		if rErr, ok := err.(*internal.RetrieveError); ok {
			return nil, (*RetrieveError)(rErr)
		}
		return nil, err
	}
	return tokenFromInternal(tk), nil
}

// RetrieveError is the error returned when the token endpoint returns a
// non-2XX HTTP status code.
type RetrieveError struct {
	Response *http.Response
	// Body is the body that was consumed by reading Response.Body.
	// It may be truncated.
	Body []byte
}

func (r *RetrieveError) Error() string {
	return fmt.Sprintf("oauth2: cannot fetch token: %v\nResponse: %s", r.Response.Status, r.Body)
}
