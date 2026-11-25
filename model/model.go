package model

import (
	"goirc/db"
	"goirc/util"

	"github.com/jmoiron/sqlx"
)

var DB *sqlx.DB

func init() {
	DB = db.Open(util.Getenv("SQLITE_DB"))
}

func Close() {
	_ = DB.Close()
}

type ChannelNick struct {
	Channel   string
	Nick      string
	Present   bool
	UpdatedAt string `db:"updated_at"`
}
