// Package oauth provides support for making
// simplified OAuth requests
// to pixlpark.ru api
package oauth

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/egorka-gh/pixlpark/pixlpark/oauth/internal"
	"github.com/go-kit/kit/log"
)

// NoContext is the default context you should supply if not using
// your own context.Context (see https://golang.org/x/net/context).
//
// Deprecated: Use context.Background() or context.TODO() instead.
var NoContext = context.TODO()

// Config describes a typical 3-legged OAuth2 flow, with both the
// client application information and the server's endpoint URLs.
// For the client credentials 2-legged OAuth2 flow, see the clientcredentials
// package (https://golang.org/x/oauth2/clientcredentials).
type Config struct {
	BaseURL string

	// ClientID is the application's ID.
	PublicKey string

	// ClientSecret is the application's secret.
	PrivateKey string

	// Endpoint contains the resource server's token endpoint
	// URLs. These are constants specific to each server and are
	// often available via site-specific packages, such as
	// google.Endpoint or github.Endpoint.
	Endpoint Endpoint

	Logger log.Logger
}

// A TokenSource is anything that can return a token.
type TokenSource interface {
	// Token returns a token or an error.
	// Token must be safe for concurrent use by multiple goroutines.
	// The returned Token must not be modified.
	Token() (*Token, error)

	//Reset source, make all token's invalid
	//next Token() call makes full fetch
	Reset()
}

// Endpoint represents an OAuth 2.0 provider's authorization and token
// endpoint URLs.
type Endpoint struct {
	RequestURL string
	TokenURL   string
	RefreshURL string
}

// An AuthCodeOption is passed to Config.AuthCodeURL.
type AuthCodeOption interface {
	setValue(url.Values)
}

type setParam struct{ k, v string }

func (p setParam) setValue(m url.Values) { m.Set(p.k, p.v) }

/*
// SetAuthURLParam builds an AuthCodeOption which passes key/value parameters
// to a provider's authorization endpoint.
func SetAuthURLParam(key, value string) AuthCodeOption {
	return setParam{key, value}
}

// AuthCodeURL returns a URL to OAuth 2.0 provider's consent page
// that asks for permissions for the required scopes explicitly.
//
// State is a token to protect the user from CSRF attacks. You must
// always provide a non-empty string and validate that it matches the
// the state query parameter on your redirect callback.
// See http://tools.ietf.org/html/rfc6749#section-10.12 for more info.
//
// Opts may include AccessTypeOnline or AccessTypeOffline, as well
// as ApprovalForce.
// It can also be used to pass the PKCE challenge.
// See https://www.oauth.com/oauth2-servers/pkce/ for more info.
func (c *Config) AuthCodeURL(state string, opts ...AuthCodeOption) string {
	var buf bytes.Buffer
	buf.WriteString(c.Endpoint.AuthURL)
	v := url.Values{
		"response_type": {"code"},
		"client_id":     {c.ClientID},
	}
	if c.RedirectURL != "" {
		v.Set("redirect_uri", c.RedirectURL)
	}
	if len(c.Scopes) > 0 {
		v.Set("scope", strings.Join(c.Scopes, " "))
	}
	if state != "" {
		// TODO(light): Docs say never to omit state; don't allow empty.
		v.Set("state", state)
	}
	for _, opt := range opts {
		opt.setValue(v)
	}
	if strings.Contains(c.Endpoint.AuthURL, "?") {
		buf.WriteByte('&')
	} else {
		buf.WriteByte('?')
	}
	buf.WriteString(v.Encode())
	return buf.String()
}
*/

// PasswordCredentialsToken converts a resource owner username and password
// pair into a token.
// see Get Request Token  && Get Access Token   http://api.pixlpark.com/
//
// The provided context optionally controls which HTTP client is used. See the HTTPClient variable.
func (c *Config) PasswordCredentialsToken(ctx context.Context) (*Token, error) {
	//get requestToken (simplified code obtaining)
	tk, err := retrieveToken(ctx, c, c.Endpoint.RequestURL, url.Values{})
	if err != nil {
		return nil, err
	}
	if tk.RequestToken == "" {
		return nil, errors.New("oauth: server response missing RequestToken")
	}

	//get access token
	v := url.Values{
		"oauth_token": {tk.RequestToken},
		"grant_type":  {"api"},
		"username":    {c.PublicKey},
		"password":    {c.genPassword(tk.RequestToken)},
	}

	t, err := retrieveToken(ctx, c, c.Endpoint.TokenURL, v)
	if err != nil {
		return nil, err
	}
	if t.AccessToken == "" {
		return nil, errors.New("oauth: server response missing AccessToken")
	}
	t.RequestToken = tk.RequestToken
	return t, nil
}

func (c *Config) genPassword(requestToken string) string {
	//TODO err checking
	var buffer bytes.Buffer
	//requestToken + c.PrivateKey
	buffer.WriteString(requestToken)
	buffer.WriteString(c.PrivateKey)
	h := sha1.New()
	h.Write(buffer.Bytes())
	bs := h.Sum(nil)
	return fmt.Sprintf("%x", bs)
}

/*
// Exchange converts an authorization code into a token.
//
// It is used after a resource provider redirects the user back
// to the Redirect URI (the URL obtained from AuthCodeURL).
//
// The provided context optionally controls which HTTP client is used. See the HTTPClient variable.
//
// The code will be in the *http.Request.FormValue("code"). Before
// calling Exchange, be sure to validate FormValue("state").
//
// Opts may include the PKCE verifier code if previously used in AuthCodeURL.
// See https://www.oauth.com/oauth2-servers/pkce/ for more info.
func (c *Config) Exchange(ctx context.Context, code string, opts ...AuthCodeOption) (*Token, error) {
	v := url.Values{
		"grant_type": {"authorization_code"},
		"code":       {code},
	}
	if c.RedirectURL != "" {
		v.Set("redirect_uri", c.RedirectURL)
	}
	for _, opt := range opts {
		opt.setValue(v)
	}
	return retrieveToken(ctx, c, v)
}
*/

// Client returns an HTTP client using the provided token.
// The token will auto-refresh as necessary. The underlying
// HTTP transport will be obtained using the provided context.
// The returned client and its Transport should not be modified.
func (c *Config) Client(ctx context.Context, t *Token) *http.Client {
	return NewClient(ctx, c.TokenSource(ctx, t))
}

//Log wraps log.logger
func (c *Config) Log(keyvals ...interface{}) error {
	if c.Logger != nil {
		return c.Logger.Log(keyvals...)
	}
	return nil
}

// TokenSource returns a TokenSource that returns t until t expires,
// automatically refreshing it as necessary using the provided context.
//
// Most users will use Config.Client instead.
func (c *Config) TokenSource(ctx context.Context, t *Token) TokenSource {
	tkf := &tokenFetcher{
		ctx:  ctx,
		conf: c,
	}
	tkr := &tokenRefresher{
		ctx:  ctx,
		conf: c,
	}
	if t != nil {
		tkf.requestToken = t.RequestToken
		tkr.refreshToken = t.RefreshToken
	}
	return &reuseTokenSource{
		t:         t,
		refresher: tkr,
		fetcher:   tkf,
		logger:    c,
	}
}

// tokenFetcher is a TokenSource that fetch new access/refresh token  using public and private key
// gets request token (if not already has it) and then new access/refresh token pair
type tokenFetcher struct {
	ctx          context.Context // used to get HTTP requests
	conf         *Config
	requestToken string
}

func (tf *tokenFetcher) Token() (*Token, error) {

	if tf.requestToken != "" {
		//short trip
		if tk, err := tf.getAccessToken(); err == nil {
			return tk, nil
		}
		//something goes wrong, reset request token
		tf.requestToken = ""
	}
	//full trip
	if err := tf.getRequestToken(); err != nil {
		return nil, err
	}
	return tf.getAccessToken()
}

func (tf *tokenFetcher) Reset() {
	tf.requestToken = ""
}

func (tf *tokenFetcher) getRequestToken() error {
	//get requestToken (simplified code obtaining)
	tf.conf.Log("tokenFetcher", "trying to get RequestToken")

	tk, err := retrieveToken(tf.ctx, tf.conf, tf.conf.Endpoint.RequestURL, url.Values{})
	if err != nil {
		return err
	}
	if tk.RequestToken == "" {
		return errors.New("oauth: server response missing RequestToken")
	}
	tf.conf.Log("RequestToken", tk.RequestToken)
	tf.requestToken = tk.RequestToken
	return nil
}

func (tf *tokenFetcher) getAccessToken() (*Token, error) {
	tf.conf.Log("tokenFetcher", "trying to get AccessToken")

	//get access token
	v := url.Values{
		"oauth_token": {tf.requestToken},
		"grant_type":  {"api"},
		"username":    {tf.conf.PublicKey},
		"password":    {tf.conf.genPassword(tf.requestToken)},
	}

	t, err := retrieveToken(tf.ctx, tf.conf, tf.conf.Endpoint.TokenURL, v)
	if err != nil {
		return nil, err
	}
	if t.AccessToken == "" {
		return nil, errors.New("oauth: server response missing AccessToken")
	}
	return t, nil
}

// tokenRefresher is a TokenSource that makes "grant_type"=="refresh_token"
// HTTP requests to renew a token using a RefreshToken.
type tokenRefresher struct {
	ctx          context.Context // used to get HTTP requests
	conf         *Config
	refreshToken string
}

// WARNING: Token is not safe for concurrent access, as it
// updates the tokenRefresher's refreshToken field.
// Within this package, it is used by reuseTokenSource which
// synchronizes calls to this method with its own mutex.
func (tf *tokenRefresher) Token() (*Token, error) {
	if tf.refreshToken == "" {
		return nil, errors.New("oauth: token expired and refresh token is not set")
	}

	tk, err := retrieveToken(tf.ctx, tf.conf, tf.conf.Endpoint.RefreshURL, url.Values{
		"refreshToken": {tf.refreshToken},
	})

	if err != nil {
		return nil, err
	}
	if tf.refreshToken != tk.RefreshToken {
		tf.refreshToken = tk.RefreshToken
	}
	return tk, err
}

func (tf *tokenRefresher) SetRefreshToken(newToken string) {
	if newToken != "" && tf.refreshToken != newToken {
		tf.refreshToken = newToken
	}
}

func (tf *tokenRefresher) Reset() {
	//noop
}

// reuseTokenSource is a TokenSource that holds a single token in memory
// and validates its expiry before each call to retrieve it with
// Token. If it's expired, it will be auto-refreshed using the
// new TokenSource.
type reuseTokenSource struct {
	fetcher   TokenSource // called when t nil or t refresh token is expired.
	refresher TokenSource // called when is access token expired.
	logger    log.Logger

	mu sync.Mutex // guards t
	t  *Token
}

// Token returns the current token if it's still valid, else will
// refresh the current token (using r.Context for HTTP client
// information) and return the new one.
func (s *reuseTokenSource) Token() (tk *Token, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		if tk != nil {
			s.logger.Log("reuseTokenSource.Token()", tk.rawBody)
		}
		if err != nil {
			s.logger.Log("reuseTokenSource err", err.Error())
		}
	}()
	if s.t.Valid() {
		s.logger.Log("reuseTokenSource", "Token still valid")
		return s.t, nil
	}
	if s.t.CanRefresh() {
		//refresh token
		s.logger.Log("reuseTokenSource", "Trying to refresh Token")
		if t, err := s.refresher.Token(); err == nil {
			s.t = t
			return t, nil
		}
		//something goes wrong, keep going
	}
	//get new token
	s.logger.Log("reuseTokenSource", "Trying to fetch Token")
	t, err := s.fetcher.Token()
	if err != nil {
		return nil, err
	}
	//update refresh token
	if rt, ok := s.refresher.(*tokenRefresher); ok {
		rt.SetRefreshToken(t.RefreshToken)
	}
	s.t = t
	return t, nil
}

//Reset set token = nil  and clear request token
func (s *reuseTokenSource) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Log("reuseTokenSource", "Reset")
	s.t = nil
	s.fetcher.Reset()
	s.refresher.Reset()
}

// StaticTokenSource returns a TokenSource that always returns the same token.
// Because the provided token t is never refreshed, StaticTokenSource is only
// useful for tokens that never expire.
func StaticTokenSource(t *Token) TokenSource {
	return staticTokenSource{t}
}

// staticTokenSource is a TokenSource that always returns the same Token.
type staticTokenSource struct {
	t *Token
}

func (s staticTokenSource) Token() (*Token, error) {
	return s.t, nil
}

func (s staticTokenSource) Reset() {
	//noop
}

// HTTPClient is the context key to use with golang.org/x/net/context's
// WithValue function to associate an *http.Client value with a context.
var HTTPClient internal.ContextKey

// NewClient creates an *http.Client from a Context and TokenSource.
// The returned client is not valid beyond the lifetime of the context.
//
// Note that if a custom *http.Client is provided via the Context it
// is used only for token acquisition and is not used to configure the
// *http.Client returned from NewClient.
//
// As a special case, if src is nil, a non-OAuth2 client is returned
// using the provided context. This exists to support related OAuth2
// packages.
func NewClient(ctx context.Context, src TokenSource) *http.Client {
	if src == nil {
		return internal.ContextClient(ctx)
	}
	//TODO add timeout??
	//timeout := time.Duration(20 * time.Second)
	return &http.Client{
		Timeout: time.Second * 20,
		Transport: &Transport{
			Base:   internal.ContextClient(ctx).Transport,
			Source: ReuseTokenSource(nil, src),
		},
	}
}

// ReuseTokenSource returns a TokenSource which repeatedly returns the
// same token as long as it's valid, starting with t.
// When its cached token is invalid, a new token is obtained from src.
//
// ReuseTokenSource is typically used to reuse tokens from a cache
// (such as a file on disk) between runs of a program, rather than
// obtaining new tokens unnecessarily.
//
// The initial token t may be nil, in which case the TokenSource is
// wrapped in a caching version if it isn't one already. This also
// means it's always safe to wrap ReuseTokenSource around any other
// TokenSource without adverse effects.
func ReuseTokenSource(t *Token, src TokenSource) TokenSource {
	// Don't wrap a reuseTokenSource in itself. That would work,
	// but cause an unnecessary number of mutex operations.
	// Just build the equivalent one.
	if rt, ok := src.(*reuseTokenSource); ok {
		if t == nil {
			// Just use it directly.
			return rt
		}
		return &reuseTokenSource{
			t:         t,
			refresher: rt.refresher,
			fetcher:   rt.fetcher,
			logger:    rt.logger,
		}
	}

	return &reuseTokenSource{
		t:         t,
		refresher: src,
		fetcher:   src,
	}
}
