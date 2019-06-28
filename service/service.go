package service

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

//http://api.pixlpark.com/

//APIVersion implemented api version
const APIVersion = "1.0"

// PPService describes the pixlpark service.
type PPService interface {
	CountOrders(ctx context.Context, statuses []string) (int, error)
	// GetOrders(ctx context.Context, statuses []string, userID, shippingID, take, skip int) (int, error)
}

//Date is time.Time, used to Unmarshal custom date format
type Date time.Time

//UnmarshalJSON  Unmarshal custom date format
//TODO write test
//TODO уточнить формат у PP
func (d *Date) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	//expected format
	//"/Date(1331083326130)/"
	//check
	if len(b) < 11 || string(b[0:6]) != "/Date(" || string(b[len(b)-2:len(b)]) != ")/" {
		//wrong format
		return fmt.Errorf("Wrong date format in %s", string(b[:]))
	}
	//get seconds
	t1, err := strconv.ParseInt(string(b[6:len(b)-(2+3)]), 10, 64)
	if err != nil {
		return err
	}
	//get milisec??
	t2, err := strconv.ParseInt(string(b[len(b)-(2+3):len(b)-2]), 10, 64)
	if err != nil {
		return err
	}

	*d = Date(time.Unix(t1, t2*1000).UTC())
	return nil
}

// Order represent pp order
type Order struct {
	ID              string          `json:"Id"`
	CustomID        string          `json:"CustomId"`
	SourceOrderID   string          `json:"SourceOrderId"`
	Title           string          `json:"Title"`
	TrackingURL     string          `json:"TrackingUrl"`
	TrackingNumber  string          `json:"TrackingNumber"`
	Status          string          `json:"Status"`
	RenderStatus    string          `json:"RenderStatus"`
	PaymentStatus   string          `json:"PaymentStatus"`
	DeliveryAddress DeliveryAddress `json:"DeliveryAddress"`
	Shipping        Shipping        `json:"Shipping"`
	CommentsCount   int             `json:"CommentsCount"`
	DownloadLink    string          `json:"DownloadLink"`
	PreviewImageURL string          `json:"PreviewImageUrl"`
	Price           float64         `json:"Price"`
	DiscountPrice   float64         `json:"DiscountPrice"`
	DeliveryPrice   float64         `json:"DeliveryPrice"`
	TotalPrice      float64         `json:"TotalPrice"`
	PaidPrice       float64         `json:"PaidPrice"`
	UserID          int             `json:"UserId"`
	DiscountID      int             `json:"DiscountId"`
	DateCreated     Date            `json:"DateCreated"`
	DateModified    Date            `json:"DateModified"`
}

// DeliveryAddress represent pp DeliveryAddress
type DeliveryAddress struct {
	ZipCode      string `json:"ZipCode"`
	AddressLine1 string `json:"AddressLine1"`
	AddressLine2 string `json:"AddressLine2"`
	Description  string `json:"Description"`
	City         string `json:"City"`
	Country      string `json:"Country"`
	FullName     string `json:"FullName"`
	Phone        string `json:"Phone"`
}

// Shipping represent pp Shipping
type Shipping struct {
	ID           int    `json:"Id"`
	Title        string `json:"Title"`
	Phone        string `json:"Phone"`
	Email        string `json:"Email"`
	ShippingType string `json:"ShippingType"`
}
