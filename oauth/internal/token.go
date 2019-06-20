// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/context/ctxhttp"
)

// Token represents the credentials used to authorize
// the requests to access protected resources on the OAuth  api.pixlpark.com
//
// This type is a mirror of oauth2.Token and exists to break
// an otherwise-circular dependency. Other internal packages
// should convert this Token into an oauth2.Token before use.
type Token struct {

	//Starting token /oauth/requesttoken
	RequestToken string

	// AccessToken is the token that authorizes and authenticates
	// the requests.
	AccessToken string

	// TokenType is the type of token.
	// The Type method returns either this or "Bearer", the default.
	TokenType string

	// RefreshToken is a token that's used by the application
	// (as opposed to the user) to refresh the access token
	// if it expires.
	RefreshToken string

	// Expiry is the optional expiration time of the access token.
	//
	// If zero, TokenSource implementations will reuse the same
	// token forever and RefreshToken or equivalent
	// mechanisms for that TokenSource will not be used.
	Expiry time.Time

	//RawBody is unparsed body
	RawBody string
}

// tokenJSON is the struct representing the HTTP response from OAuth2
// providers returning a token in JSON form.
type tokenJSON struct {
	RequestToken string         `json:"RequestToken"`
	AccessToken  string         `json:"AccessToken"`
	RefreshToken string         `json:"RefreshToken"`
	ExpiresIn    expirationTime `json:"Expires"` // at least PayPal returns string, while most return number
	Success      bool           `json:"Success"`
	Error        string         `json:"Error"`
}

func (e *tokenJSON) expiry() (t time.Time) {
	if v := e.ExpiresIn; v != 0 {
		return time.Now().Add(time.Duration(v) * time.Second)
	}
	return
}

type expirationTime int32

func (e *expirationTime) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var n json.Number
	err := json.Unmarshal(b, &n)
	if err != nil {
		return err
	}
	i, err := n.Int64()
	if err != nil {
		return err
	}
	if i > math.MaxInt32 {
		i = math.MaxInt32
	}
	*e = expirationTime(i)
	return nil
}

// newTokenRequest returns a new *http.Request to retrieve a new token
// from tokenURL using the provided parameters.
// unlike oauth use GET request
func newTokenRequest(tokenURL string, v url.Values) (*http.Request, error) {
	req, err := http.NewRequest("GET", tokenURL, nil)
	if err != nil {
		return nil, err
	}
	if v != nil {
		// need??
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		//TODO this replace query, if url has some - rewrite to add
		req.URL.RawQuery = v.Encode()
	}
	return req, nil
}

func cloneURLValues(v url.Values) url.Values {
	v2 := make(url.Values, len(v))
	for k, vv := range v {
		v2[k] = append([]string(nil), vv...)
	}
	return v2
}

//RetrieveToken makes doTokenRoundTrip
func RetrieveToken(ctx context.Context, tokenURL string, v url.Values) (*Token, error) {
	req, err := newTokenRequest(tokenURL, v)
	if err != nil {
		return nil, err
	}
	token, err := doTokenRoundTrip(ctx, req)
	/*
		// Don't overwrite `RefreshToken` with an empty value
		// if this was a token refreshing request.
		if token != nil && token.RefreshToken == "" {
			token.RefreshToken = v.Get("refresh_token")
		}
	*/
	return token, err
}

func doTokenRoundTrip(ctx context.Context, req *http.Request) (*Token, error) {
	r, err := ctxhttp.Do(ctx, ContextClient(ctx), req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1<<20))
	r.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("oauth: cannot fetch token: %v", err)
	}
	if code := r.StatusCode; code < 200 || code > 299 {
		return nil, &RetrieveError{
			Response: r,
			Body:     body,
		}
	}

	var token *Token
	var tj tokenJSON
	if err = json.Unmarshal(body, &tj); err != nil {
		return nil, err
	}
	if tj.Success == false {
		if tj.Error == "" {
			err = errors.New("oauth: server response Success = false")
		} else {
			err = errors.New(tj.Error)
		}
		return nil, err
	}
	token = &Token{
		RequestToken: tj.RequestToken,
		AccessToken:  tj.AccessToken,
		TokenType:    "",
		RefreshToken: tj.RefreshToken,
		Expiry:       tj.expiry(),
		RawBody:      string(body[:]),
	}

	return token, nil
}

//RetrieveError struct to get err
type RetrieveError struct {
	Response *http.Response
	Body     []byte
}

func (r *RetrieveError) Error() string {
	return fmt.Sprintf("oauth: cannot fetch token: %v\nResponse: %s", r.Response.Status, r.Body)
}
