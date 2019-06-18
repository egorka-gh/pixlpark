// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"mime"
	"net/http"
	"net/url"
	"strconv"
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

	// Raw optionally contains extra metadata from the server
	// when updating a token.
	Raw interface{}
}

// tokenJSON is the struct representing the HTTP response from OAuth2
// providers returning a token in JSON form.
type tokenJSON struct {
	RequestToken string         `json:"RequestToken"`
	AccessToken  string         `json:"AccessToken"`
	RefreshToken string         `json:"RefreshToken"`
	ExpiresIn    expirationTime `json:"Expires"` // at least PayPal returns string, while most return number
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

	/*
		if authStyle == AuthStyleInParams {
			v = cloneURLValues(v)
			if clientID != "" {
				v.Set("client_id", clientID)
			}
			if clientSecret != "" {
				v.Set("client_secret", clientSecret)
			}
		}
		req, err := http.NewRequest("POST", tokenURL, strings.NewReader(v.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if authStyle == AuthStyleInHeader {
			req.SetBasicAuth(url.QueryEscape(clientID), url.QueryEscape(clientSecret))
		}
		return req, nil
	*/
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
	// Don't overwrite `RefreshToken` with an empty value
	// if this was a token refreshing request.
	if token != nil && token.RefreshToken == "" {
		token.RefreshToken = v.Get("refresh_token")
	}
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
	//TODO remove urlencoded, expected JSON
	content, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	switch content {
	case "application/x-www-form-urlencoded", "text/plain":
		vals, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, err
		}
		token = &Token{
			AccessToken:  vals.Get("access_token"),
			TokenType:    vals.Get("token_type"),
			RefreshToken: vals.Get("refresh_token"),
			Raw:          vals,
		}
		e := vals.Get("expires_in")
		expires, _ := strconv.Atoi(e)
		if expires != 0 {
			token.Expiry = time.Now().Add(time.Duration(expires) * time.Second)
		}
	default:
		var tj tokenJSON
		if err = json.Unmarshal(body, &tj); err != nil {
			return nil, err
		}
		token = &Token{
			RequestToken: tj.RequestToken,
			AccessToken:  tj.AccessToken,
			TokenType:    "",
			RefreshToken: tj.RefreshToken,
			Expiry:       tj.expiry(),
			Raw:          make(map[string]interface{}),
		}
		json.Unmarshal(body, &token.Raw) // no error checks for optional fields
	}
	/*
		if token.AccessToken == "" && token.RequestToken == "" {
			return nil, errors.New("oauth: server response missing access_token")
		}
	*/
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
