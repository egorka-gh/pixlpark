// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package oauth

import (
	"errors"
	"io"
	"net/http"
	"sync"
)

// Transport is an http.RoundTripper that makes OAuth 2.0 HTTP requests,
// wrapping a base RoundTripper and adding an Authorization header
// with a token from the supplied Sources.
//
// Transport is a low-level mechanism. Most code will use the
// higher-level Config.Client method instead.
type Transport struct {
	// Source supplies the token to add to outgoing requests'
	// Authorization headers.
	Source TokenSource

	// Base is the base RoundTripper used to make HTTP requests.
	// If nil, http.DefaultTransport is used.
	Base http.RoundTripper

	mu     sync.Mutex                      // guards modReq
	modReq map[*http.Request]*http.Request // original -> modified
}

// RoundTrip authorizes and authenticates the request with an
// access token from Transport's Source.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqBodyClosed := false
	if req.Body != nil {
		defer func() {
			if !reqBodyClosed {
				req.Body.Close()
			}
		}()
	}

	if t.Source == nil {
		return nil, errors.New("oauth2: Transport's Source is nil")
	}
	token, err := t.Source.Token()
	if err != nil {
		return nil, err
	}

	req2 := cloneRequest(req) // per RoundTripper contract
	//TODO remove auth header
	token.SetAuthHeader(req2)
	token.SetAuthURLParametr(req2)
	t.setModReq(req, req2)
	res, err := t.base().RoundTrip(req2)
	if err != nil {
		t.Source.Reset()
		reqBodyClosed = true
		t.setModReq(req, nil)
		return nil, err
	}

	//in case 401 respose reset t.Source and do trip again
	//TODO req body already closed, weakness? just reset t.Source?
	if res.StatusCode == 401 {
		t.Source.Reset()
		token, err = t.Source.Token()
		if err != nil {
			return nil, err
		}
		req2 = cloneRequest(req) // per RoundTripper contract

		/*
		//TODO  Rewind the body if we're able to. (like base transport do)
		if req.GetBody != nil {
			newReq := *req
			var err error
			newReq.Body, err = req.GetBody()
			if err != nil {
				return nil, err
			}
			req = &newReq
		}
		*/

		token.SetAuthURLParametr(req2)
		t.setModReq(req, req2)
		res, err = t.base().RoundTrip(req2)
	}

	// req.Body is assumed to have been closed by the base RoundTripper.
	reqBodyClosed = true

	if err != nil {
		t.setModReq(req, nil)
		return nil, err
	}
	res.Body = &onEOFReader{
		rc: res.Body,
		fn: func() { t.setModReq(req, nil) },
	}
	return res, nil
}

// CancelRequest cancels an in-flight request by closing its connection.
func (t *Transport) CancelRequest(req *http.Request) {
	type canceler interface {
		CancelRequest(*http.Request)
	}
	if cr, ok := t.base().(canceler); ok {
		t.mu.Lock()
		modReq := t.modReq[req]
		delete(t.modReq, req)
		t.mu.Unlock()
		cr.CancelRequest(modReq)
	}
}

func (t *Transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func (t *Transport) setModReq(orig, mod *http.Request) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.modReq == nil {
		t.modReq = make(map[*http.Request]*http.Request)
	}
	if mod == nil {
		delete(t.modReq, orig)
	} else {
		t.modReq[orig] = mod
	}
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	//don't reuse connection
	r2.Close = true
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	// copy url
	url2 := *r.URL
	r2.URL = &url2
	return r2
}

type onEOFReader struct {
	rc io.ReadCloser
	fn func()
}

func (r *onEOFReader) Read(p []byte) (n int, err error) {
	n, err = r.rc.Read(p)
	if err == io.EOF {
		r.runFunc()
	}
	return
}

func (r *onEOFReader) Close() error {
	err := r.rc.Close()
	r.runFunc()
	return err
}

func (r *onEOFReader) runFunc() {
	if fn := r.fn; fn != nil {
		fn()
		r.fn = nil
	}
}
