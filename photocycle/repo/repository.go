package repo

import (
	"context"
	"strings"

	cycle "github.com/egorka-gh/pixlpark/photocycle"
	"github.com/jmoiron/sqlx"
)

type basicRepository struct {
	db     *sqlx.DB
	Source int
}

//New creates new Repository
func New(connection string) (cycle.Repository, error) {
	rep, _, err := NewTest(connection)
	return rep, err
}

//NewTest creates new Repository, expect mysql connection sqlx.DB
func NewTest(connection string) (cycle.Repository, *sqlx.DB, error) {
	var db *sqlx.DB
	db, err := sqlx.Connect("mysql", connection)
	if err != nil {
		return nil, nil, err
	}

	return &basicRepository{
		db: db,
	}, db, nil
}

func (b *basicRepository) Close() {
	b.db.Close()
}

func (b *basicRepository) CreateOrder(ctx context.Context, o cycle.Order) error {

	//var ssql = "SELECT source, table_name, latest_version FROM cnv_version WHERE source = ? ORDER BY syncorder"
	var sb strings.Builder
	//INSERT IGNORE  ??
	sb.WriteString("INSERT INTO orders (id, source, src_id, src_date, data_ts, state, state_date, group_id, ftp_folder, fotos_num, client_id, production)")
	sb.WriteString(" VALUES (?, ?, ?, ?, ?, ?, NOW(), ?, ?, ?, ?, ?)")
	var ssql = sb.String()
	_, err := b.db.ExecContext(ctx, ssql, o.ID, o.Source, o.SourceID, o.SourceDate, o.DataTS, o.State, o.GroupID, o.FtpFolder, o.FotosNum, o.ClientID, o.Production)
	return err
	/*
		//ignore ErrNoRows ??
		if err != nil && err == sql.ErrNoRows {
			return res, nil
		}
	*/
}

func (b *basicRepository) ClearGroup(ctx context.Context, group int, keepID string) error {
	sql := "DELETE FROM orders WHERE group_id = ? AND ID != ?"
	_, err := b.db.ExecContext(ctx, sql, group, keepID)
	return err
}

func (b *basicRepository) SetGroupState(ctx context.Context, state, group int, keepID string) error {
	sql := "UPDATE orders SET state = ? WHERE group_id = ? AND ID != ?"
	_, err := b.db.ExecContext(ctx, sql, state, group, keepID)
	return err
}

func (b *basicRepository) LoadOrder(ctx context.Context, id string) (cycle.Order, error) {
	var res cycle.Order
	ssql := "SELECT id, source, src_id, src_date, data_ts, state, state_date, group_id, ftp_folder, fotos_num, client_id, production FROM orders WHERE id = ?"
	err := b.db.GetContext(ctx, &res, ssql, id)
	return res, err
}

func (b *basicRepository) LogState(ctx context.Context, orderID string, state int, message string) error {
	ssql := "INSERT INTO state_log (order_id, state, state_date, comment) VALUES (?, ?, NOW(), LEFT(?, 250))"
	_, err := b.db.ExecContext(ctx, ssql, orderID, state, message)
	return err
}

func (b *basicRepository) SetOrderState(ctx context.Context, orderID string, state int) error {
	ssql := "UPDATE orders o SET o.state = ?, o.state_date = Now() WHERE o.id = ?"
	_, err := b.db.ExecContext(ctx, ssql, state, orderID)
	return err
}

func (b *basicRepository) LoadAlias(ctx context.Context, alias string) (cycle.Alias, error) {
	var res cycle.Alias
	ssql := "SELECT id, synonym, book_type, synonym_type, (SELECT IFNULL(MAX(1), 0) FROM book_pg_template bpt WHERE bpt.book = bs.id AND bpt.book_part IN (1, 3, 4, 5)) has_cover FROM book_synonym bs WHERE bs.src_type = 4 AND bs.synonym = ? ORDER BY bs.synonym_type DESC"
	err := b.db.GetContext(ctx, &res, ssql, alias)
	return res, err
}

func (b *basicRepository) AddExtraInfo(ctx context.Context, ei cycle.OrderExtraInfo) error {
	var sb strings.Builder
	//INSERT IGNORE  ??
	sb.WriteString("INSERT INTO order_extra_info (id, endpaper, interlayer, cover, format, corner_type, kaptal, cover_material, books, sheets, date_in, book_thickness, group_id, remark, paper, calc_alias, calc_title, weight)")
	sb.WriteString(" VALUES (?, LEFT(?, 100), LEFT(?, 100), LEFT(?, 250), LEFT(?, 250), LEFT(?, 100), LEFT(?, 100), LEFT(?, 250), ?, ?, ?, ?, ?, LEFT(?, 250), LEFT(?, 250), LEFT(?, 50), LEFT(?, 250), ?)")
	var sql = sb.String()
	_, err := b.db.ExecContext(ctx, sql, ei.ID, ei.EndPaper, ei.InterLayer, ei.Cover, ei.Format, ei.CornerType, ei.Kaptal, ei.CoverMaterial, ei.Books, ei.Sheets, ei.Date, ei.BookThickness, ei.GroupID, ei.Remark, ei.Paper, ei.Alias, ei.Title, ei.Weight)
	return err
}

func (b *basicRepository) GetGroupState(ctx context.Context, baseID string, group int) (cycle.GroupState, error) {
	var res cycle.GroupState
	sql := "SELECT IFNULL(MAX(IF(o.id = ?, o.state, 0)), 0) basestate, IFNULL(MAX(IF(o.id = ?, 0, o.state)), 0) childstate FROM orders o WHERE o.group_id = ?"
	err := b.db.GetContext(ctx, &res, sql, baseID, baseID, group)
	return res, err
}

func (b *basicRepository) LoadBaseOrderByState(ctx context.Context, source, state int) (cycle.Order, error) {
	var res cycle.Order
	ssql := "SELECT id, source, src_id, src_date, data_ts, state, state_date, group_id, ftp_folder, fotos_num, client_id, production FROM orders WHERE source = ? AND id LIKE '%@' AND state = ? LIMIT 1"
	err := b.db.GetContext(ctx, &res, ssql, source, state)
	return res, err
}

func (b *basicRepository) LoadBaseOrderByChildState(ctx context.Context, source, baseState, childState int) ([]cycle.Order, error) {
	res := []cycle.Order{}
	var sb strings.Builder
	sb.WriteString("SELECT o.id, o.source, o.src_id, o.src_date, o.data_ts, o.state, o.state_date, o.group_id, o.ftp_folder, o.fotos_num, o.client_id, o.production")
	sb.WriteString(" FROM orders o")
	sb.WriteString(" WHERE o.source = ? AND o.id LIKE '%@' AND o.state = ? AND EXISTS (SELECT 1 FROM orders o1 WHERE o1.group_id = o.group_id AND o1.state = ?)")
	sql := sb.String()
	err := b.db.SelectContext(ctx, &res, sql, source, baseState, childState)

	return res, err
}
