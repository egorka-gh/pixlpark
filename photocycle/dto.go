package photocycle

import (
	"context"
	"fmt"
	"strconv"
	"time"

	pp "github.com/egorka-gh/pixlpark/pixlpark/service"
)

// Repository describes the persistence on dto
type Repository interface {
	//ListSource(ctx context.Context, source string) ([]Source, error)
	CreateOrder(ctx context.Context, o Order) (Order, error)
	LoadOrder(ctx context.Context, id string) (Order, error)
	LogState(ctx context.Context, orderID string, state int, message string) error
	SetOrderState(ctx context.Context, orderID string, state int) error
	Close()
}

//Order represents the Order of db object
type Order struct {
	ID          string    `json:"id" db:"id"`
	Source      int       `json:"source" db:"source"`
	SourceID    string    `json:"src_id" db:"src_id"`
	SourceDate  time.Time `json:"src_date" db:"src_date"`
	DataTS      time.Time `json:"data_ts" db:"data_ts"`
	State       int       `json:"state" db:"state"`
	StateDate   time.Time `json:"state_date" db:"state_date"`
	FtpFolder   string    `json:"ftp_folder" db:"ftp_folder"`
	LocalFolder string    `json:"local_folder" db:"local_folder"`
	FotosNum    int       `json:"fotos_num" db:"fotos_num"`
	GroupID     int       `json:"group_id" db:"group_id"`
	ClientID    int       `json:"client_id" db:"client_id"`
	Production  int       `json:"production" db:"production"`
}

//OrderExtraInfo represents the OrderExtraInfo of db object
type OrderExtraInfo struct {
	ID      string `json:"id" db:"id"`
	GroupID int    `json:"group_id" db:"group_id"`
	Weight  int    `json:"weight" db:"weight"`
}

//PrintGroup represents the PrintGroup of db object
type PrintGroup struct {
	ID        string    `json:"id" db:"id"`
	OrderID   string    `json:"order_id" db:"order_id"`
	State     int       `json:"state" db:"state"`
	StateDate time.Time `json:"state_date" db:"state_date"`
	Width     int       `json:"width" db:"width"`
	Height    int       `json:"height" db:"height"`
	Paper     int       `json:"paper" db:"paper"`
	Frame     int       `json:"frame" db:"frame"`
	Cutting   int       `json:"cutting" db:"cutting"`
	Path      string    `json:"path" db:"path"`
	Alias     string    `json:"alias" db:"alias"`
	FileNum   int       `json:"file_num" db:"file_num"`
	BookType  int       `json:"book_type" db:"book_type"`
	BookPart  int       `json:"book_part" db:"book_part"`
	BookNum   int       `json:"book_num" db:"book_num"`
	SheetNum  int       `json:"sheet_num" db:"sheet_num"`
	IsPDF     bool      `json:"is_pdf" db:"is_pdf"`
	IsDuplex  bool      `json:"is_duplex" db:"is_duplex"`
	Butt      int       `json:"butt" db:"butt"`
}

//TODO add print_group_file

//FromPPOrder converts PP order to photocycle order
func FromPPOrder(o pp.Order, source int, sufix string) Order {
	g, err := strconv.Atoi(o.ID)
	if err != nil {
		g = 0
	}
	return Order{
		ID:         fmt.Sprintf("%d_%s%s", source, o.ID, sufix),
		Source:     source,
		SourceID:   fmt.Sprintf("%s%s", o.ID, sufix),
		SourceDate: time.Time(o.DateCreated),
		DataTS:     time.Time(o.DateModified),
		GroupID:    g,
		ClientID:   o.UserID, //??
	}
}
