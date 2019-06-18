package oauth

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockTransport struct {
	rt func(req *http.Request) (resp *http.Response, err error)
}

func (t *mockTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	return t.rt(req)
}

func newConf(url string) *Config {
	return &Config{
		PublicKey:  "PublicKey",
		PrivateKey: "PrivateKey",
		Endpoint: Endpoint{
			RequestURL: url + "/request",
			RefreshURL: url + "/refresh",
			TokenURL:   url + "/token",
		},
	}
}

//invalid test & conf.PasswordCredentialsToken not used
func TestPasswordCredentialsTokenRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		expected := ""
		/*
			if r.URL.String() != expected {
				t.Errorf("URL = %q; want %q", r.URL, expected)
			}
			headerAuth := r.Header.Get("Authorization")
			expected = "Basic Q0xJRU5UX0lEOkNMSUVOVF9TRUNSRVQ="
			if headerAuth != expected {
				t.Errorf("Authorization header = %q; want %q", headerAuth, expected)
			}
		*/
		headerContentType := r.Header.Get("Content-Type")
		expected = "application/x-www-form-urlencoded"
		if headerContentType != expected {
			t.Errorf("Content-Type header = %q; want %q", headerContentType, expected)
		}
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed reading request body: %s.", err)
		}
		expected = "grant_type=password&password=password1&scope=scope1+scope2&username=user1"
		if string(body) != expected {
			t.Errorf("res.Body = %q; want %q", string(body), expected)
		}
		w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		w.Write([]byte("access_token=90d64460d14870c08c81352a05dedd3465940a7c&scope=user&token_type=bearer"))
	}))
	defer ts.Close()
	conf := newConf(ts.URL)
	tok, err := conf.PasswordCredentialsToken(context.Background())
	if err != nil {
		t.Error(err)
	}
	if !tok.Valid() {
		t.Fatalf("Token invalid. Got: %#v", tok)
	}
	expected := "90d64460d14870c08c81352a05dedd3465940a7c"
	if tok.AccessToken != expected {
		t.Errorf("AccessToken = %q; want %q", tok.AccessToken, expected)
	}
	expected = "bearer"
	if tok.TokenType != expected {
		t.Errorf("TokenType = %q; want %q", tok.TokenType, expected)
	}
}

func TestTokenFetchRequest(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/somethingelse" {
			tk := r.URL.Query().Get("oauth_token")
			if tk != "12345" {
				t.Errorf("Unexpected oauth_token %q", tk)
			}

			return
		}
		if r.URL.String() == "/request" {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"RequestToken": "rtoken"}`)
			return
		}
		if r.URL.Path != "/token" {
			//TODO herpoime
			t.Errorf("Unexpected token request URL %q", r.URL)
		}
		headerContentType := r.Header.Get("Content-Type")
		if headerContentType != "application/x-www-form-urlencoded" {
			t.Errorf("Unexpected Content-Type header %q", headerContentType)
		}
		/*
			body, _ := ioutil.ReadAll(r.Body)
			if string(body) != "grant_type=refresh_token&refresh_token=REFRESH_TOKEN" {
				t.Errorf("Unexpected refresh token payload %q", body)
			}
		*/
		params := r.URL.RawQuery
		if params != "grant_type=api&oauth_token=rtoken&password=333a4c19df851472c77ac824cfd603ec2b3159e2&username=PublicKey" {
			t.Errorf("Unexpected refresh token payload %q", params)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"AccessToken": "12345", "RefreshToken": "bar", "Expires": 50}`)
	}))
	defer ts.Close()
	conf := newConf(ts.URL)
	c := conf.Client(context.Background(), nil)

	//gets new token request + access
	_, err := c.Get(ts.URL + "/somethingelse")
	if err != nil {
		t.Errorf("Error fetching token %q", err.Error())
	}
	//refresh token
	_, err = c.Get(ts.URL + "/somethingelse")
	if err != nil {
		t.Errorf("Error refreshing token %q", err.Error())
	}

}
