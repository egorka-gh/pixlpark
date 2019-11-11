package service

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
)

//NewRouter creates chi.Mux
func NewRouter() *chi.Mux {
	r := chi.NewRouter()
	//r.Use(middleware.RequestID)
	//r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	//r.Use(middleware.URLFormat)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	})

	r.Route("/api", func(r chi.Router) {
		/* 		r.With(paginate).Get("/", ListArticles)
		   		r.Post("/", CreateArticle)       // POST /articles
		   		r.Get("/search", SearchArticles) // GET /articles/search

		   		r.Route("/{articleID}", func(r chi.Router) {
		   			r.Use(ArticleCtx)            // Load the *Article on the request context
		   			r.Get("/", GetArticle)       // GET /articles/123
		   			r.Put("/", UpdateArticle)    // PUT /articles/123
		   			r.Delete("/", DeleteArticle) // DELETE /articles/123
		   		})

		   		// GET /articles/whats-up
		   		r.With(ArticleCtx).Get("/{articleSlug:[a-z-]+}", GetArticle)
		*/
		r.Route("/order/{orderID}", func(r chi.Router) {
		})
	})

	return r
}
