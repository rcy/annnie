package bug

import (
	"goirc/internal/responder"
	"net/url"
)

func Handle(params responder.Responder) error {
	var title string
	if len(params.Matches()) > 1 {
		title = params.Match(1)
	}

	params.Privmsgf(params.Target(), "%s: https://github.com/rcy/annnie/issues/new?labels=bug&title=%s", params.Nick(), url.QueryEscape(title))
	return nil
}
