package oauth

import (
	"context"
	"io"
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

func TestTokenFetchRequest(t *testing.T) {
	var requests, tokens, refreshes, othes int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch path := r.URL.Path; path {
		case "/somethingelse":
			othes++
			if othes > 2 {
				t.Errorf("Unexpected call to %q expected %q got %q", path, 1, othes)
			}
			tk := r.URL.Query().Get("oauth_token")
			if tk != "12345" {
				t.Errorf("Unexpected oauth_token %q; path %q", tk, path)
			}
		case "/request":
			requests++
			if requests > 1 {
				t.Errorf("Unexpected call to %q expected %q got %q", path, 1, requests)
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"RequestToken": "rtoken"}`)
		case "/refresh":
			refreshes++
			if refreshes > 0 {
				t.Errorf("Unexpected call to %q expected %q got %q", path, 0, refreshes)
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"AccessToken": "67890", "RefreshToken": "bar2", "Expires": 100}`)
		case "/token":
			tokens++
			if tokens > 2 {
				t.Errorf("Unexpected call to %q expected %q got %q", path, 1, tokens)
			}
			headerContentType := r.Header.Get("Content-Type")
			if headerContentType != "application/x-www-form-urlencoded" {
				t.Errorf("Unexpected Content-Type header %q", headerContentType)
			}
			params := r.URL.RawQuery
			if params != "grant_type=api&oauth_token=rtoken&password=333a4c19df851472c77ac824cfd603ec2b3159e2&username=PublicKey" {
				t.Errorf("Unexpected refresh token payload %q", params)
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"AccessToken": "12345", "RefreshToken": "bar", "Expires": 5}`)
		default:
			t.Errorf("Unexpected request URL %q", r.URL)
		}
	}))
	defer ts.Close()
	conf := newConf(ts.URL)
	c := conf.Client(context.Background(), nil)

	//gets new token request + access
	_, err := c.Get(ts.URL + "/somethingelse")
	if err != nil {
		t.Errorf("Error fetching token %q", err.Error())
	}
	//refresh expere so must get new token
	_, err = c.Get(ts.URL + "/somethingelse")
	if err != nil {
		t.Errorf("Error refreshing token %q", err.Error())
	}

}

func TestTokenRefreshRequest(t *testing.T) {
	var requests, tokens, refreshes, othes int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch path := r.URL.Path; path {
		case "/somethingelse":
			othes++
			if othes > 2 {
				t.Errorf("Unexpected call to %q expected %q got %q", path, 1, othes)
			}
			tk := r.URL.Query().Get("oauth_token")
			if tk != "12345" && tk != "67890" {
				t.Errorf("Unexpected oauth_token %q; path %q", tk, path)
			}
		case "/request":
			requests++
			if requests > 1 {
				t.Errorf("Unexpected call to %q expected %q got %q", path, 1, requests)
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"RequestToken": "rtoken"}`)
		case "/refresh":
			refreshes++
			if refreshes > 1 {
				t.Errorf("Unexpected call to %q expected %q got %q", path, 1, refreshes)
			}
			param := r.URL.Query().Get("refreshToken")
			if param != "bar" {
				t.Errorf("Unexpected refresh token, expected %q got %q", "bar", param)
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"AccessToken": "67890", "RefreshToken": "bar2", "Expires": 100}`)
		case "/token":
			tokens++
			if tokens > 1 {
				t.Errorf("Unexpected call to %q expected %q got %q", path, 1, tokens)
			}
			headerContentType := r.Header.Get("Content-Type")
			if headerContentType != "application/x-www-form-urlencoded" {
				t.Errorf("Unexpected Content-Type header %q", headerContentType)
			}
			params := r.URL.RawQuery
			if params != "grant_type=api&oauth_token=rtoken&password=333a4c19df851472c77ac824cfd603ec2b3159e2&username=PublicKey" {
				t.Errorf("Unexpected get access token query %q", params)
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"AccessToken": "12345", "RefreshToken": "bar", "Expires": 50}`)
		default:
			t.Errorf("Unexpected request URL %q", r.URL)
		}
	}))
	defer ts.Close()
	conf := newConf(ts.URL)
	c := conf.Client(context.Background(), nil)

	//gets new token request + access
	_, err := c.Get(ts.URL + "/somethingelse")
	if err != nil {
		t.Errorf("Error fetching token %q", err.Error())
	}
	//refresh expere so must get new token
	_, err = c.Get(ts.URL + "/somethingelse")
	if err != nil {
		t.Errorf("Error refreshing token %q", err.Error())
	}

}

func TestTokenValidRequest(t *testing.T) {
	var requests, tokens, refreshes, othes int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch path := r.URL.Path; path {
		case "/somethingelse":
			othes++
			if othes > 2 {
				t.Errorf("Unexpected call to %q expected %q got %q", path, 1, othes)
			}
			tk := r.URL.Query().Get("oauth_token")
			if tk != "12345" {
				t.Errorf("Unexpected oauth_token %q; path %q", tk, path)
			}
		case "/request":
			requests++
			if requests > 1 {
				t.Errorf("Unexpected call to %q expected %q got %q", path, 1, requests)
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"RequestToken": "rtoken"}`)
		case "/refresh":
			refreshes++
			if refreshes > 0 {
				t.Errorf("Unexpected call to %q expected %q got %q", path, 0, refreshes)
			}
			param := r.URL.Query().Get("refreshToken")
			if param != "bar" {
				t.Errorf("Unexpected refresh token, expected %q got %q", "bar", param)
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"AccessToken": "67890", "RefreshToken": "bar2", "Expires": 100}`)
		case "/token":
			tokens++
			if tokens > 1 {
				t.Errorf("Unexpected call to %q expected %q got %q", path, 1, tokens)
			}
			headerContentType := r.Header.Get("Content-Type")
			if headerContentType != "application/x-www-form-urlencoded" {
				t.Errorf("Unexpected Content-Type header %q", headerContentType)
			}
			params := r.URL.RawQuery
			if params != "grant_type=api&oauth_token=rtoken&password=333a4c19df851472c77ac824cfd603ec2b3159e2&username=PublicKey" {
				t.Errorf("Unexpected get access token query %q", params)
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"AccessToken": "12345", "RefreshToken": "bar", "Expires": 100}`)
		default:
			t.Errorf("Unexpected request URL %q", r.URL)
		}
	}))
	defer ts.Close()
	conf := newConf(ts.URL)
	c := conf.Client(context.Background(), nil)

	//gets new token request + access
	_, err := c.Get(ts.URL + "/somethingelse")
	if err != nil {
		t.Errorf("Error fetching token %q", err.Error())
	}
	//refresh expere so must get new token
	_, err = c.Get(ts.URL + "/somethingelse")
	if err != nil {
		t.Errorf("Error refreshing token %q", err.Error())
	}

}
