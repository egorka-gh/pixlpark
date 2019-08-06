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

func (b *basicRepository) CreateOrder(ctx context.Context, o cycle.Order) (cycle.Order, error) {

	//var ssql = "SELECT source, table_name, latest_version FROM cnv_version WHERE source = ? ORDER BY syncorder"
	var sb strings.Builder
	sb.WriteString("INSERT IGNORE INTO orders (id, source, src_id, src_date, data_ts, state, state_date, group_id, ftp_folder, fotos_num, client_id, production)")
	sb.WriteString(" VALUES (?, ?, ?, ?, ?, ?, NOW(), ?, ?, ?, ?, ?)")
	var ssql = sb.String()
	_, err := b.db.ExecContext(ctx, ssql, o.ID, o.Source, o.SourceID, o.SourceDate, o.DataTS, cycle.StateLoadWaite, o.GroupID, o.FtpFolder, o.FotosNum, o.ClientID, o.Production)
	if err != nil {
		return cycle.Order{}, err
	}
	/*
		//ignore ErrNoRows ??
		if err != nil && err == sql.ErrNoRows {
			return res, nil
		}
	*/
	return b.LoadOrder(ctx, o.ID)
}

func (b *basicRepository) LoadOrder(ctx context.Context, id string) (cycle.Order, error) {
	var res cycle.Order
	ssql := "SELECT id, source, src_id, src_date, data_ts, state, state_date, group_id, ftp_folder, fotos_num, client_id, production FROM orders WHERE id = ?"
	err := b.db.GetContext(ctx, &res, ssql, id)
	/*
		//ignore ErrNoRows ??
		if err != nil && err == sql.ErrNoRows {
			return res, nil
		}
	*/
	return res, err
}
