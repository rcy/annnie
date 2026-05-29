package responder

import "context"

type Responder interface {
	Context() context.Context
	Privmsgf(string, string, ...interface{})
	Target() string
	Nick() string
	Match(num int) string
	Matches() []string
	Msg() string
}
