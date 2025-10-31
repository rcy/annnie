package commit

import (
	"database/sql"
	"fmt"
	"goirc/model"
)

var Rev = "main"

func URL() (string, error) {
	var id int64
	err := model.DB.Get(&id, `select id from revs where sha = ?`, Rev)
	if err == sql.ErrNoRows {
		_, err = model.DB.Exec(`insert into revs(sha) values(?)`, Rev)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("https://github.com/rcy/annnie/commit/%s", Rev), nil
	}
	return "", err
}
