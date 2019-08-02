package repo

import (
	cycle "github.com/egorka-gh/pixlpark/photocycle"
	"github.com/jmoiron/sqlx"
)

type basicRepository struct {
	db *sqlx.DB
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
