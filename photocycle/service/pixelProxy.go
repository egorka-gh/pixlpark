package service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"

	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

var ppClient pp.PPService

//NewRouter creates chi.Mux
func NewRouter(client pp.PPService) *chi.Mux {
	ppClient = client
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
			//r.Use(OrderCtx)       // Load the *Order on the request context
			r.With(OrderCtx).Get("/", GetOrder)  // GET /order/123 (REST standard)
			r.With(OrderCtx).Post("/", GetOrder) // cycle allways uses Post, so route it as GET
			r.Post("/status", SetOrderState)     //set order status, not PUT - cycle allways uses Post
			//4 debug
			//r.Get("/status", SetOrderState)
		})

		//get order and transform to MailPackage payload as it expect cycle
		r.Route("/mailpackage/{orderID}", func(r chi.Router) {
			r.Use(OrderCtx) // Load the *Order on the request context
			r.Get("/", GetMailpackage)
			r.Post("/", GetMailpackage) // cycle allways uses Post, so route it as GET
		})
	})
	//mount net/http/pprof
	r.Mount("/debug", middleware.Profiler())
	return r
}

// OrderCtx middleware is used to load an Order object from pixelpark.
// In case pixelpark returns some error, we stop here and return a error.
func OrderCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var order pp.Order
		var err error

		if orderID := chi.URLParam(r, "orderID"); orderID != "" {
			order, err = ppClient.GetOrder(r.Context(), orderID)
		} else {
			render.Render(w, r, ErrNotFound)
			return
		}

		if err != nil {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		ctx := context.WithValue(r.Context(), "order", &order)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

//SetOrderState redirects method to pixel api
func SetOrderState(w http.ResponseWriter, r *http.Request) {

	orderID := chi.URLParam(r, "orderID")
	if orderID == "" {
		render.Render(w, r, ErrNotFound)
		return
	}
	//parse status from post form
	status := r.FormValue("status")
	if status == "" {
		render.Render(w, r, ErrNotFound)
		return
	}

	//render.Render(w, r, &BaseResponse{Result: message(fmt.Sprintf("ID: %s; State: %s", orderID, status))})
	if err := ppClient.SetOrderStatus(r.Context(), orderID, status, true); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}
	if err := render.Render(w, r, &BaseResponse{Result: message("OK")}); err != nil {
		render.Render(w, r, ErrRender(err))
		return
	}

}

// GetOrder returns the specific Order. It just
// fetches the Order right off the context
func GetOrder(w http.ResponseWriter, r *http.Request) {
	// Assume if we've reach this far, we can access the order
	// context because this handler is a child of the OrderCtx
	// middleware. The worst case, the recoverer middleware will save us.
	order := r.Context().Value("order").(*pp.Order)

	if err := render.Render(w, r, NewOrderResponse(order)); err != nil {
		render.Render(w, r, ErrRender(err))
		return
	}
}

// GetMailpackage returns the specific Order transformed to MailPackage.
func GetMailpackage(w http.ResponseWriter, r *http.Request) {
	// Assume if we've reach this far, we can access the order
	// context because this handler is a child of the OrderCtx
	// middleware. The worst case, the recoverer middleware will save us.
	order := r.Context().Value("order").(*pp.Order)

	if err := render.Render(w, r, NewMailPackageResponse(order)); err != nil {
		render.Render(w, r, ErrRender(err))
		return
	}
}

// BaseResponse is the base response for cycle
type BaseResponse struct {
	Result render.Renderer `json:"result"`
}

//Render implement Renderer
func (b *BaseResponse) Render(w http.ResponseWriter, r *http.Request) error {
	// Pre-processing before a response is marshalled and sent across the wire
	return nil
}

type message string

func (m message) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// OrderResponse is the response payload for the Order data model.
type OrderResponse struct {
	ID    string `json:"id"`
	State string `json:"status"`
	//TODO production, group?
}

//Render implement Renderer
func (o *OrderResponse) Render(w http.ResponseWriter, r *http.Request) error {
	// Pre-processing before a response is marshalled and sent across the wire
	return nil
}

//NewOrderResponse creates new OrderResponse
func NewOrderResponse(order *pp.Order) *BaseResponse {
	resp := &OrderResponse{ID: order.ID, State: order.Status}
	return &BaseResponse{Result: resp}
}

//MailPackageResponse represents the MailPackage dto for cycle web client
type MailPackageResponse struct {
	ID            string            `json:"id"`
	IDName        string            `json:"number"`
	ClientID      int               `json:"member_id"`
	ExecutionDate string            `json:"execution_date"`
	DeliveryID    int               `json:"delivery_id"`
	DeliveryName  string            `json:"delivery_title"`
	StateName     string            `json:"status_text"`
	Properties    map[string]string `json:"address"`
	//TODO messages?
	//TODO barcodes?
}

//Render implement Renderer
func (m *MailPackageResponse) Render(w http.ResponseWriter, r *http.Request) error {
	// Pre-processing before a response is marshalled and sent across the wire
	return nil
}

//NewMailPackageResponse creates new MailPackageResponse
func NewMailPackageResponse(order *pp.Order) *BaseResponse {
	resp := &MailPackageResponse{
		ID:           order.ID,
		IDName:       order.ID,
		ClientID:     order.UserID,
		DeliveryID:   order.Shipping.ID,
		DeliveryName: order.Shipping.Title,
		StateName:    order.Status,
		Properties:   make(map[string]string),
	}

	//add properties
	resp.Properties["lastname"] = order.DeliveryAddress.FullName
	resp.Properties["phone"] = order.DeliveryAddress.Phone
	//resp.Properties["email"] = TODO get in from User?
	//resp.Properties["passport"] = TODO
	//resp.Properties["passport_date"] = TODO
	resp.Properties["postal"] = order.DeliveryAddress.ZipCode
	resp.Properties["region"] = order.DeliveryAddress.State
	//resp.Properties["district"] = TODO
	resp.Properties["city"] = order.DeliveryAddress.City
	resp.Properties["street"] = order.DeliveryAddress.AddressLine1
	resp.Properties["home"] = order.DeliveryAddress.AddressLine2
	resp.Properties["debt_sum"] = fmt.Sprintf("%.2f", order.TotalPrice-order.PaidPrice)

	return &BaseResponse{Result: resp}
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
