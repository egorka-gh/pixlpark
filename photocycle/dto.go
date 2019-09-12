package photocycle

import (
	"context"
	"time"
)

// Repository describes the persistence on dto
type Repository interface {
	//ListSource(ctx context.Context, source string) ([]Source, error)
	CreateOrder(ctx context.Context, o Order) error
	LoadOrder(ctx context.Context, id string) (Order, error)
	LogState(ctx context.Context, orderID string, state int, message string) error
	SetOrderState(ctx context.Context, orderID string, state int) error
	LoadAlias(ctx context.Context, alias string) (Alias, error)
	ClearGroup(ctx context.Context, group int, keepID string) error
	SetGroupState(ctx context.Context, state, group int, keepID string) error
	AddExtraInfo(ctx context.Context, ei OrderExtraInfo) error
	GetGroupState(ctx context.Context, baseID string, group int) (GroupState, error)
	LoadBaseOrderByState(ctx context.Context, source, state int) (Order, error)
	LoadBaseOrderByChildState(ctx context.Context, source, baseState, childState int) ([]Order, error)
	FillOrders(ctx context.Context, orders []Order) error
	Close()
}

//Alias represents the book_synonym db object
type Alias struct {
	ID       int    `json:"id" db:"id"`
	Alias    string `json:"synonym" db:"synonym"`
	Type     int    `json:"book_type" db:"book_type"`
	SubType  int    `json:"synonym_type" db:"synonym_type"`
	HasCover bool   `json:"has_cover" db:"has_cover"`
}

//Order represents the Order db object
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

	//4 internal use
	ExtraInfo   OrderExtraInfo
	HasCover    bool
	PrintGroups []PrintGroup
}

//GroupState is dto for orders states by GroupID
type GroupState struct {
	BaseState  int `json:"basestate" db:"basestate"`
	ChildState int `json:"childstate" db:"childstate"`
}

//OrderExtraInfo represents the OrderExtraInfo of db object
type OrderExtraInfo struct {
	ID            string    `json:"id" db:"id"`
	GroupID       int       `json:"group_id" db:"group_id"`
	EndPaper      string    `json:"endpaper" db:"endpaper"`
	InterLayer    string    `json:"interlayer" db:"interlayer"`
	Cover         string    `json:"cover" db:"cover"`
	Format        string    `json:"format" db:"format"`
	CornerType    string    `json:"corner_type" db:"corner_type"`
	Kaptal        string    `json:"kaptal" db:"kaptal"`
	CoverMaterial string    `json:"cover_material" db:"cover_material"`
	Weight        int       `json:"weight" db:"weight"`
	Books         int       `json:"books" db:"books"`
	Sheets        int       `json:"sheets" db:"sheets"`
	Date          time.Time `json:"date_in" db:"date_in"`
	BookThickness float32   `json:"book_thickness" db:"book_thickness"`
	Remark        string    `json:"remark" db:"remark"`
	Paper         string    `json:"paper" db:"paper"`
	Alias         string    `json:"calc_alias" db:"calc_alias"`
	Title         string    `json:"calc_title" db:"calc_title"`
}

//PrintGroup represents the PrintGroup of db object
type PrintGroup struct {
	ID         string    `json:"id" db:"id"`
	OrderID    string    `json:"order_id" db:"order_id"`
	State      int       `json:"state" db:"state"`
	StateDate  time.Time `json:"state_date" db:"state_date"`
	Width      int       `json:"width" db:"width"`
	Height     int       `json:"height" db:"height"`
	Paper      int       `json:"paper" db:"paper"`
	Frame      int       `json:"frame" db:"frame"`
	Correction int       `json:"correction" db:"correction"`
	Cutting    int       `json:"cutting" db:"cutting"`
	Path       string    `json:"path" db:"path"`
	Alias      string    `json:"alias" db:"alias"`
	FileNum    int       `json:"file_num" db:"file_num"`
	BookType   int       `json:"book_type" db:"book_type"`
	BookPart   int       `json:"book_part" db:"book_part"`
	BookNum    int       `json:"book_num" db:"book_num"`
	SheetNum   int       `json:"sheet_num" db:"sheet_num"`
	IsPDF      bool      `json:"is_pdf" db:"is_pdf"`
	IsDuplex   bool      `json:"is_duplex" db:"is_duplex"`
	Prints     int       `json:"prints" db:"prints"`
	Butt       int       `json:"butt" db:"butt"`

	//4 internal use
	Files []PrintGroupFile
}

//PrintGroupFile represents the print_group_file of db object
type PrintGroupFile struct {
	PrintGroupID string `json:"print_group" db:"print_group"`
	FileName     string `json:"file_name" db:"file_name"`
	PrintQtty    int    `json:"prt_qty" db:"prt_qty"`
	Book         int    `json:"book_num" db:"book_num"`
	Page         int    `json:"page_num" db:"page_num"`
	Caption      string `json:"caption" db:"caption"`
	BookPart     int    `json:"book_part" db:"book_part"`
}
