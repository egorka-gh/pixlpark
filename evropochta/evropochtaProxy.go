package evropochta

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/go-kit/kit/log"
)

//HandlerConfig to create mux
type HandlerConfig struct {
	Client Evropochta
	Logger log.Logger
}

type proxy struct {
	mux    *chi.Mux
	config *HandlerConfig
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mux.ServeHTTP(w, r)
}

//New creats http.Handler
func NewHandler(config *HandlerConfig) http.Handler {
	return &proxy{
		config: config,
		mux:    createRouter(config),
	}
}

//NewRouter creates chi.Mux
func createRouter(config *HandlerConfig) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	})

	r.Route("/api", func(r chi.Router) {
		r.Route("/sticker/{trackNum}", func(r chi.Router) {
			r.Get("/", config.GetSticker)
		})
	})
	return r
}

func (c *HandlerConfig) GetSticker(w http.ResponseWriter, r *http.Request) {

	trackNum := chi.URLParam(r, "trackNum")
	if trackNum == "" {
		render.Render(w, r, ErrNotFound)
		return
	}

	sticker, err := c.Client.GetPostSticker(r.Context(), trackNum)
	if err != nil {
		c.Logger.Log(err.Error())
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", sticker.FileName))
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(sticker.FileData)
	if err != nil {
		c.Logger.Log(err.Error())
	}
}

//--
// Error response payloads & renderers
//--

// ErrResponse renderer type for handling all sorts of errors.
//
// In the best case scenario, the excellent github.com/pkg/errors package
// helps reveal information on the error, setting it on Err, and in the Render()
// method, using it to set the application-specific error code in AppCode.
type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status"`          // user-level status message
	AppCode    int64  `json:"code,omitempty"`  // application-specific error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

//Render implement Renderer
func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	if e.HTTPStatusCode == 0 {
		e.HTTPStatusCode = 400
	}
	if e.ErrorText == "" && e.Err != nil {
		e.ErrorText = e.Err.Error()
	}
	render.Status(r, e.HTTPStatusCode)
	return nil
}

//ErrInvalidRequest creates ErrInvalidRequest response from error
func ErrInvalidRequest(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: 400,
		StatusText:     "Invalid request.",
		ErrorText:      err.Error(),
	}
}

//ErrRender creates ErrRender response from error
func ErrRender(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: 422,
		StatusText:     "Error rendering response.",
		ErrorText:      err.Error(),
	}
}

//ErrNotFound creates ErrNotFound response
var ErrNotFound = &ErrResponse{HTTPStatusCode: 404, StatusText: "Resource not found."}

//ErrNotConfigured creates ErrNotConfigured response
var ErrNotConfigured = &ErrResponse{HTTPStatusCode: 501, StatusText: "Not configured."}
