/*
Package service is client for http://api.pixlpark.com/
*/
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

//http://api.pixlpark.com/

//APIVersion implemented api version
const APIVersion = "1.0"

// PPService describes the pixlpark service.
type PPService interface {
	CountOrders(ctx context.Context, statuses []string) (int, error)
	GetOrders(ctx context.Context, status string, userID, shippingID, take, skip int) ([]Order, error)
	GetOrder(ctx context.Context, id string) (Order, error)
	GetOrderItems(ctx context.Context, orderID string) ([]OrderItem, error)
	SetOrderStatus(ctx context.Context, id, status string, notify bool) error
	AddOrderComment(ctx context.Context, id, email, comment string) error
}

//Date is time.Time, used to Unmarshal custom date format
type Date time.Time

//UnmarshalJSON  Unmarshal custom date format
//TODO уточнить формат у PP
func (d *Date) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	//TODO escaped / "DateCreated":"\/Date(1564133280000)\/" ??
	//unmarshal to string??
	var s string
	json.Unmarshal(b, &s)
	b = []byte(s)
	//expected format
	//"/Date(1331083326130)/"
	//"DateCreated": "/Date(1562920440000)/",
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

func (d Date) format(s string) string {
	t := time.Time(d)
	return t.Format(s)
}

//String implementing Stringer interface
func (d Date) String() string {
	return d.format(time.RFC3339)
}

// Order represent pp order
type Order struct {
	ID                   string          `json:"Id"`
	PhotolabID           int             `json:"PhotolabId"`
	CustomID             string          `json:"CustomId"`
	SourceOrderID        string          `json:"SourceOrderId"`
	ManagerID            string          `json:"ManagerId"`
	ProductionID         int             `json:"AssignedToId"`
	Title                string          `json:"Title"`
	TrackingURL          string          `json:"TrackingUrl"`
	TrackingNumber       string          `json:"TrackingNumber"`
	Status               string          `json:"Status"`
	RenderStatus         string          `json:"RenderStatus"`
	PaymentStatus        string          `json:"PaymentStatus"`
	DeliveryAddress      DeliveryAddress `json:"DeliveryAddress"`
	Shipping             Shipping        `json:"Shipping"`
	CommentsCount        int             `json:"CommentsCount"`
	DownloadLink         string          `json:"DownloadLink"`
	PreviewImageURL      string          `json:"PreviewImageUrl"`
	Price                float64         `json:"Price"`
	DiscountPrice        float64         `json:"DiscountPrice"`
	DeliveryPrice        float64         `json:"DeliveryPrice"`
	TotalPrice           float64         `json:"TotalPrice"`
	PaidPrice            float64         `json:"PaidPrice"`
	UserID               int             `json:"UserId"`
	UserCompanyAccountID int             `json:"UserCompanyAccountId"`
	DiscountID           int             `json:"DiscountId"` //?? no in response
	DiscountTitle        string          `json:"DiscountTitle"`
	DateCreated          Date            `json:"DateCreated,string"`
	DateModified         Date            `json:"DateModified,string"`
	//Items                []OrderItem     `json:"-"`
}

// DeliveryAddress represent pp DeliveryAddress
type DeliveryAddress struct {
	ZipCode      string `json:"ZipCode"`
	AddressLine1 string `json:"AddressLine1"`
	AddressLine2 string `json:"AddressLine2"`
	Description  string `json:"Description"`
	City         string `json:"City"`
	Country      string `json:"Country"`
	State        string `json:"State"`
	FullName     string `json:"FullName"`
	Phone        string `json:"Phone"`
}

// Shipping represent pp Shipping
type Shipping struct {
	ID           int    `json:"Id"`
	Title        string `json:"Title"`
	Phone        string `json:"Phone"`
	Email        string `json:"Email"`
	ShippingType string `json:"ShippingType"` //?? no in response
	Type         string `json:"Type"`
}

// OrderItem represent pp Order Item
type OrderItem struct {
	ID               int               `json:"Id"`
	OrderID          int               `json:"OrderId"`
	MaterialID       int               `json:"MaterialId"` //product id
	Name             string            `json:"Name"`
	Description      string            `json:"Description"`
	Quantity         int               `json:"Quantity"`
	ItemPrice        float64           `json:"ItemPrice"`
	AdditionalPrice  float64           `json:"AdditionalPrice"`
	CustomWorkPrice  float64           `json:"CustomWorkPrice"`
	TotalPrice       float64           `json:"TotalPrice"`
	DiscountPrice    float64           `json:"DiscountPrice"`
	PageCount        int               `json:"PageCount"`
	SkuItems         []OrderItemSku    `json:"SkuItems"`
	PreviewImages    []string          `json:"PreviewImages"`
	Options          []OrderItemOption `json:"Options"`
	AdditionalFields map[string]string `json:"AdditionalFields"`
	DirectoryName    string            `json:"DirectoryName"`
	Comment          string            `json:"Comment"`
	Sizes            OrderItemSizes    `json:"EditorOutput"`
	TotalWeight      float64           `json:"TotalWeight"`
	skuMap           map[string]string
}

//Sku returns map of SkuItems (Name->Value)
//it joins OrderItem and Options SkuItems, Options.SkuItems owerwrites Values vs same key if any
func (i OrderItem) Sku() map[string]string {
	if i.skuMap != nil {
		return i.skuMap
	}
	i.skuMap = make(map[string]string)
	for _, sku := range i.SkuItems {
		i.skuMap[strings.ToLower(sku.Name)] = sku.Value
	}
	for _, opt := range i.Options {
		for _, sku := range opt.SkuItems {
			i.skuMap[strings.ToLower(sku.Name)] = sku.Value
		}
	}
	return i.skuMap
}

// OrderItemOption represent pp Order Item Option
type OrderItemOption struct {
	Title           string         `json:"Title"`
	Description     string         `json:"Description"`
	Price           float64        `json:"Price"`
	PriceFormatType string         `json:"PriceFormatType"`
	SkuItems        []OrderItemSku `json:"SkuItems"`
}

// OrderItemSizes represent pp Order Item EditorOutput
type OrderItemSizes struct {
	Height    float64 `json:"Height"`
	Width     float64 `json:"Width"`
	Thickness float64 `json:"Thickness"`
}

// OrderItemSku represent pp Order Item SKU
type OrderItemSku struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}
