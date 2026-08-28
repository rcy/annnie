package model

import (
	"fmt"
	"goirc/config"
	"goirc/internal/idstr"
)

func (n Note) Link() (string, error) {
	if config.Get().AnonymizeLinks != "" {
		str, err := idstr.Encode(n.ID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s/%s", config.Get().RootURL, str), nil
	}

	return n.Text.String, nil
}
