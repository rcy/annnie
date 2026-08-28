package model

import (
	"goirc/config"
	"goirc/db"

	"github.com/jmoiron/sqlx"
)

var DB *sqlx.DB

func init() {
	if path := config.Get().SQLiteDB; path != "" {
		DB = db.Open(path)
	}
}

func Close() {
	DB.Close()
}

type ChannelNick struct {
	Channel   string
	Nick      string
	Present   bool
	UpdatedAt string `db:"updated_at"`
}
