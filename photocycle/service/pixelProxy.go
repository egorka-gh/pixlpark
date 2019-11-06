package service

import (
	"github.com/go-chi/chi"
)

//NewRouter creates chi.Mux
func NewRouter() *chi.Mux {
	r := chi.NewRouter()

	return r
}
