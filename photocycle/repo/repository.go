package repo

import (
	"context"
	"strings"

	cycle "github.com/egorka-gh/pixlpark/photocycle"
	"github.com/jmoiron/sqlx"
)

type basicRepository struct {
	db *sqlx.DB
	//	Source   int
	readOnly bool
}

//New creates new Repository
func New(connection string, readOnly bool) (cycle.Repository, error) {
	rep, _, err := NewTest(connection, readOnly)
	return rep, err
}

//NewTest creates new Repository, expect mysql connection sqlx.DB
func NewTest(connection string, readOnly bool) (cycle.Repository, *sqlx.DB, error) {
	var db *sqlx.DB
	db, err := sqlx.Connect("mysql", connection)
	if err != nil {
		return nil, nil, err
	}

	return &basicRepository{
		db:       db,
		readOnly: readOnly,
	}, db, nil
}

func (b *basicRepository) Close() {
	b.db.Close()
}

func (b *basicRepository) CreateOrder(ctx context.Context, o cycle.Order) error {
	if b.readOnly {
		return nil
	}
	//var ssql = "SELECT source, table_name, latest_version FROM cnv_version WHERE source = ? ORDER BY syncorder"
	var sb strings.Builder
	//INSERT IGNORE  ??
	sb.WriteString("INSERT INTO orders (id, source, src_id, src_date, data_ts, state, state_date, group_id, ftp_folder, fotos_num, client_id, production)")
	sb.WriteString(" VALUES (?, ?, ?, ?, ?, ?, NOW(), ?, ?, ?, ?, ?)")
	var ssql = sb.String()
	_, err := b.db.ExecContext(ctx, ssql, o.ID, o.Source, o.SourceID, o.SourceDate, o.DataTS, o.State, o.GroupID, o.FtpFolder, o.FotosNum, o.ClientID, o.Production)
	return err
}

func (b *basicRepository) FillOrders(ctx context.Context, orders []cycle.Order) error {
	if b.readOnly {
		return nil
	}
	//insert orders
	oSQL := "INSERT INTO orders (id, source, src_id, src_date, data_ts, state, state_date, group_id, ftp_folder, fotos_num, client_id, production) VALUES "
	oVals := make([]string, 0, len(orders))
	oArgs := []interface{}{}

	xSQL := "INSERT INTO order_extra_info (id, endpaper, interlayer, cover, format, corner_type, kaptal, cover_material, books, sheets, date_in, book_thickness, group_id, remark, paper, calc_alias, calc_title, weight) VALUES "
	xVals := make([]string, 0, len(orders))
	xArgs := []interface{}{}

	pSQL := "INSERT INTO print_group (id, order_id, state, state_date, width, height, paper, frame, correction, cutting, path, alias, file_num, book_type, book_part, book_num, sheet_num, is_pdf, is_duplex, prints, butt) VALUES "
	pVals := make([]string, 0, len(orders)*2)
	pArgs := []interface{}{}

	fSQL := "INSERT INTO print_group_file (print_group, file_name, prt_qty, book_num, page_num, caption, book_part) VALUES"
	fVals := make([]string, 0, len(orders)*2*10)
	fArgs := []interface{}{}

	for _, o := range orders {
		//orders
		oVals = append(oVals, "(?, ?, ?, ?, ?, ?, NOW(), ?, ?, ?, ?, ?)")
		oArgs = append(oArgs, o.ID, o.Source, o.SourceID, o.SourceDate, o.DataTS, o.State, o.GroupID, o.FtpFolder, o.FotosNum, o.ClientID, o.Production)
		//extra info
		ei := o.ExtraInfo
		xVals = append(xVals, "(?, LEFT(?, 100), LEFT(?, 100), LEFT(?, 250), LEFT(?, 250), LEFT(?, 100), LEFT(?, 100), LEFT(?, 250), ?, ?, ?, ?, ?, LEFT(?, 250), LEFT(?, 250), LEFT(?, 50), LEFT(?, 250), ?)")
		xArgs = append(xArgs, ei.ID, ei.EndPaper, ei.InterLayer, ei.Cover, ei.Format, ei.CornerType, ei.Kaptal, ei.CoverMaterial, ei.Books, ei.Sheets, ei.Date, ei.BookThickness, ei.GroupID, ei.Remark, ei.Paper, ei.Alias, ei.Title, ei.Weight)
		//print groups
		for _, p := range o.PrintGroups {
			pVals = append(pVals, "(?, ?, ?, NOW(), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			pArgs = append(pArgs, p.ID, p.OrderID, p.State, p.Width, p.Height, p.Paper, p.Frame, p.Correction, p.Cutting, p.Path, p.Alias, p.FileNum, p.BookType, p.BookPart, p.BookNum, p.SheetNum, p.IsPDF, p.IsDuplex, p.Prints, p.Butt)
			//files
			for _, f := range p.Files {
				fVals = append(fVals, "(?, ?, ?, ?, ?, ?, ?)")
				fArgs = append(fArgs, f.PrintGroupID, f.FileName, f.PrintQtty, f.Book, f.Page, f.Caption, f.BookPart)
			}
		}
	}

	oSQL = oSQL + strings.Join(oVals, ",")
	xSQL = xSQL + strings.Join(xVals, ",")

	pSQL = pSQL + strings.Join(pVals, ",")
	fSQL = fSQL + strings.Join(fVals, ",")

	//run in transaction
	t, err := b.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	_, err = t.Exec(oSQL, oArgs...)
	if err != nil {
		t.Rollback()
		return err
	}

	_, err = t.Exec(xSQL, xArgs...)
	if err != nil {
		t.Rollback()
		return err
	}
	if len(pVals) > 0 {
		_, err = t.Exec(pSQL, pArgs...)
		if err != nil {
			t.Rollback()
			return err
		}
	}
	if len(fVals) > 0 {
		_, err = t.Exec(fSQL, fArgs...)
		if err != nil {
			t.Rollback()
			return err
		}
	}

	return t.Commit()
}

func (b *basicRepository) ClearGroup(ctx context.Context, source, group int, keepID string) error {
	if b.readOnly {
		return nil
	}
	sql := "DELETE FROM orders WHERE source =? AND group_id = ? AND ID != ?"
	_, err := b.db.ExecContext(ctx, sql, source, group, keepID)
	return err
}

func (b *basicRepository) SetGroupState(ctx context.Context, source, state, group int, keepID string) error {
	if b.readOnly {
		return nil
	}
	sql := "UPDATE orders SET state = ? WHERE source =? AND group_id = ? AND ID != ?"
	_, err := b.db.ExecContext(ctx, sql, state, source, group, keepID)
	return err
}

func (b *basicRepository) StartOrders(ctx context.Context, source, group int, skipID string) error {
	if b.readOnly {
		return nil
	}
	sql := "call pp_StartOrders(?, ?, ?)"
	_, err := b.db.ExecContext(ctx, sql, source, group, skipID)
	return err
}

func (b *basicRepository) LoadOrder(ctx context.Context, id string) (cycle.Order, error) {
	var res cycle.Order
	//ssql := "SELECT id, source, src_id, src_date, data_ts, state, state_date, group_id, ftp_folder, fotos_num, client_id, production FROM orders WHERE id = ?"
	ssql := "SELECT id, source, src_id, src_date, state, state_date, group_id, ftp_folder, fotos_num, client_id, production FROM orders WHERE id = ?"
	err := b.db.GetContext(ctx, &res, ssql, id)
	return res, err
}

func (b *basicRepository) LogState(ctx context.Context, orderID string, state int, message string) error {
	if b.readOnly {
		return nil
	}
	ssql := "INSERT INTO state_log (order_id, state, state_date, comment) VALUES (?, ?, NOW(), LEFT(?, 250))"
	_, err := b.db.ExecContext(ctx, ssql, orderID, state, message)
	return err
}

func (b *basicRepository) SetOrderState(ctx context.Context, orderID string, state int) error {
	if b.readOnly {
		return nil
	}
	ssql := "UPDATE orders o SET o.state = ?, o.state_date = Now() WHERE o.id = ?"
	_, err := b.db.ExecContext(ctx, ssql, state, orderID)
	return err
}

func (b *basicRepository) LoadAlias(ctx context.Context, alias string) (cycle.Alias, error) {
	var res cycle.Alias
	ssql := "SELECT 0 id, af.alias synonym, 0 book_type, 0 synonym_type, 0 has_cover, af.state forward FROM alias_forward af WHERE af.alias =? " +
		" UNION ALL" +
		" SELECT id, synonym, book_type, synonym_type, (SELECT IFNULL(MAX(1), 0) FROM book_pg_template bpt WHERE bpt.book = bs.id AND bpt.book_part IN (1, 3, 4, 5)) has_cover, 0 forward FROM book_synonym bs WHERE bs.src_type = 4 AND bs.synonym = ? ORDER BY synonym_type DESC"
	err := b.db.GetContext(ctx, &res, ssql, alias, alias)
	return res, err
}

func (b *basicRepository) AddExtraInfo(ctx context.Context, ei cycle.OrderExtraInfo) error {
	if b.readOnly {
		return nil
	}
	var sb strings.Builder
	//INSERT IGNORE  ??
	sb.WriteString("INSERT INTO order_extra_info (id, endpaper, interlayer, cover, format, corner_type, kaptal, cover_material, books, sheets, date_in, book_thickness, group_id, remark, paper, calc_alias, calc_title, weight)")
	sb.WriteString(" VALUES (?, LEFT(?, 100), LEFT(?, 100), LEFT(?, 250), LEFT(?, 250), LEFT(?, 100), LEFT(?, 100), LEFT(?, 250), ?, ?, ?, ?, ?, LEFT(?, 250), LEFT(?, 250), LEFT(?, 50), LEFT(?, 250), ?)")
	var sql = sb.String()
	_, err := b.db.ExecContext(ctx, sql, ei.ID, ei.EndPaper, ei.InterLayer, ei.Cover, ei.Format, ei.CornerType, ei.Kaptal, ei.CoverMaterial, ei.Books, ei.Sheets, ei.Date, ei.BookThickness, ei.GroupID, ei.Remark, ei.Paper, ei.Alias, ei.Title, ei.Weight)
	return err
}

func (b *basicRepository) GetGroupState(ctx context.Context, baseID string, source, group int) (cycle.GroupState, error) {
	var res cycle.GroupState
	sql := "SELECT IFNULL(o.group_id, 0) group_id, IFNULL(MAX(IF(o.id = ?, o.state, 0)), 0) basestate, IFNULL(MAX(IF(o.id = ?, 0, o.state)), 0) childstate, NOW() state_date FROM orders o WHERE o.source = ? AND o.group_id = ?"
	err := b.db.GetContext(ctx, &res, sql, baseID, baseID, source, group)
	return res, err
}

func (b *basicRepository) LoadBaseOrderByState(ctx context.Context, source, state int) (cycle.Order, error) {
	var res cycle.Order
	//ssql := "SELECT id, source, src_id, src_date, data_ts, state, state_date, group_id, ftp_folder, fotos_num, client_id, production FROM orders WHERE source = ? AND id LIKE '%@' AND state = ? LIMIT 1"
	ssql := "SELECT id, source, src_id, src_date, state, state_date, group_id, ftp_folder, fotos_num, client_id, production FROM orders WHERE source = ? AND id LIKE '%@' AND state = ? LIMIT 1"
	err := b.db.GetContext(ctx, &res, ssql, source, state)
	return res, err
}

func (b *basicRepository) LoadBaseOrderByChildState(ctx context.Context, source, baseState, childState int) ([]cycle.Order, error) {
	res := []cycle.Order{}
	var sb strings.Builder
	//sb.WriteString("SELECT o.id, o.source, o.src_id, o.src_date, o.data_ts, o.state, o.state_date, o.group_id, o.ftp_folder, o.fotos_num, o.client_id, o.production")
	sb.WriteString("SELECT o.id, o.source, o.src_id, o.src_date, o.state, o.state_date, o.group_id, o.ftp_folder, o.fotos_num, o.client_id, o.production")
	sb.WriteString(" FROM orders o")
	sb.WriteString(" WHERE o.source = ? AND o.id LIKE '%@' AND o.state = ? AND EXISTS (SELECT 1 FROM orders o1 WHERE o1.group_id = o.group_id AND o1.state = ?)")
	sql := sb.String()
	err := b.db.SelectContext(ctx, &res, sql, source, baseState, childState)

	return res, err
}

func (b *basicRepository) CountCurrentOrders(ctx context.Context, source int) (int, error) {
	var res int
	sql := "SELECT COUNT(DISTINCT o.group_id) FROM orders o WHERE o.state BETWEEN 100 AND 450 AND o.source = ?"
	err := b.db.GetContext(ctx, &res, sql, source)
	return res, err
}

func (b *basicRepository) GetCurrentOrders(ctx context.Context, source int) ([]cycle.GroupState, error) {
	res := []cycle.GroupState{}
	var sb strings.Builder
	sb.WriteString("SELECT o.group_id, MAX(o.state) basestate, MIN(o.state) childstate, MAX(o.state_date) state_date")
	sb.WriteString(" FROM orders o")
	sb.WriteString(" WHERE o.state BETWEEN 100 AND 450 AND o.source = ?")
	sb.WriteString(" GROUP BY o.group_id")
	sql := sb.String()
	err := b.db.SelectContext(ctx, &res, sql, source)
	return res, err
}
