package oauth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	log "github.com/go-kit/kit/log"
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
		Logger: log.NewLogfmtLogger(os.Stderr),
	}
}

func TestTokenSuccess(t *testing.T) {
	var requests, tokens, refreshes, othes int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch path := r.URL.Path; path {
		case "/somethingelse":
			othes++
			tk := r.URL.Query().Get("oauth_token")
			if tk != "12345" {
				t.Errorf("Unexpected oauth_token %q; path %q", tk, path)
			}
		case "/request":
			requests++
			if requests == 1 {
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"RequestToken": "rtoken", "Success": false, "Error": "Some error"}`)
			} else {
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"RequestToken": "rtoken", "Success": true, "Error": null}`)
			}
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
			io.WriteString(w, `{"AccessToken": "12345", "RefreshToken": "bar", "Expires": 300, "Success": true}`)
		default:
			t.Errorf("Unexpected request URL %q", r.URL)
		}
	}))
	defer ts.Close()

	conf := newConf(ts.URL)
	c := conf.Client(context.Background(), nil)

	//Success false
	_, err := c.Get(ts.URL + "/somethingelse")
	if err == nil {
		t.Error("Expected error")
	} /* else if err.Error() != "Some error" {
		t.Errorf("Get wrong err %q", err.Error())
	}*/

	//gets new token request + access
	_, err = c.Get(ts.URL + "/somethingelse")
	if err != nil {
		t.Errorf("Error refreshing token %q", err.Error())
	}

}

func TestTokenFetchRequest(t *testing.T) {
	var requests, tokens, refreshes, othes int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch path := r.URL.Path; path {
		case "/somethingelse":
			othes++
			tk := r.URL.Query().Get("oauth_token")
			if tk != "12345" {
				t.Errorf("Unexpected oauth_token %q; path %q", tk, path)
			}
		case "/request":
			requests++
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"RequestToken": "rtoken", "Success": true}`)
		case "/refresh":
			refreshes++
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"AccessToken": "67890", "RefreshToken": "bar2", "Expires": 100, "Success": true}`)
		case "/token":
			tokens++
			headerContentType := r.Header.Get("Content-Type")
			if headerContentType != "application/x-www-form-urlencoded" {
				t.Errorf("Unexpected Content-Type header %q", headerContentType)
			}
			params := r.URL.RawQuery
			if params != "grant_type=api&oauth_token=rtoken&password=333a4c19df851472c77ac824cfd603ec2b3159e2&username=PublicKey" {
				t.Errorf("Unexpected refresh token payload %q", params)
			}
			w.Header().Set("Content-Type", "application/json")
			if tokens == 2 {
				io.WriteString(w, `{"AccessToken": "12345", "RefreshToken": "bar", "Expires": 5, "Success": false}`)
			} else {
				io.WriteString(w, `{"AccessToken": "12345", "RefreshToken": "bar", "Expires": 5, "Success": true}`)

			}
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

	//"Success": false must reget request token (access + request + access)
	_, err = c.Get(ts.URL + "/somethingelse")
	if err != nil {
		t.Errorf("Error refreshing token %q", err.Error())
	}

	//refresh expere so must get new token (access)
	_, err = c.Get(ts.URL + "/somethingelse")
	if err != nil {
		t.Errorf("Error refreshing token %q", err.Error())
	}

	if othes != 3 {
		t.Errorf("Unexpected call to 'othes' expected %q got %q", 3, othes)
	}
	if requests != 2 {
		t.Errorf("Unexpected call to 'requests' expected %q got %q", 2, requests)
	}
	if refreshes != 0 {
		t.Errorf("Unexpected call to 'refreshes' expected %q got %q", 0, refreshes)
	}
	if tokens != 4 {
		t.Errorf("Unexpected call to 'tokens' expected %q got %q", 4, tokens)
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
			io.WriteString(w, `{"RequestToken": "rtoken", "Success": true}`)
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
			io.WriteString(w, `{"AccessToken": "67890", "RefreshToken": "bar2", "Expires": 100, "Success": true}`)
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
			io.WriteString(w, `{"AccessToken": "12345", "RefreshToken": "bar", "Expires": 50, "Success": true}`)
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
			io.WriteString(w, `{"RequestToken": "rtoken", "Success": true}`)
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
			io.WriteString(w, `{"AccessToken": "67890", "RefreshToken": "bar2", "Expires": 100, "Success": true}`)
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
			io.WriteString(w, `{"AccessToken": "12345", "RefreshToken": "bar", "Expires": 100, "Success": true}`)
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
	//access not expere so no request , access, refresh
	_, err = c.Get(ts.URL + "/somethingelse")
	if err != nil {
		t.Errorf("Error refreshing token %q", err.Error())
	}

}
