package bot

import (
	"context"
	"goirc/internal/responder"
	"goirc/pubsub"
	"strings"

	irc "github.com/thoj/go-ircevent"
)

type HandlerParams struct {
	ctx       context.Context
	privmsgf  func(string, string, ...interface{})
	msg       string
	nick      string
	target    string
	matches   []string
	LastEvent *irc.Event
}

func NewHandlerParams(ctx context.Context, target string, privmsgf func(string, string, ...interface{})) HandlerParams {
	return HandlerParams{ctx: ctx, target: target, privmsgf: privmsgf}
}

func (hp HandlerParams) Context() context.Context {
	if hp.ctx != nil {
		return hp.ctx
	}
	return context.Background()
}

func (hp HandlerParams) Privmsgf(target string, format string, a ...interface{}) {
	hp.privmsgf(target, format, a...)
}

func (hp HandlerParams) Target() string {
	return hp.target
}

func (hp HandlerParams) Nick() string {
	return hp.nick
}

func (hp HandlerParams) Match(num int) string {
	return strings.TrimSpace(hp.matches[num])
}

func (hp HandlerParams) Matches() []string {
	return hp.matches
}

func (hp HandlerParams) Msg() string {
	return hp.msg
}

func (hp *HandlerParams) Publish(eventName string, payload any) {
	pubsub.Publish(eventName, payload)
}

type HandlerFunction func(responder.Responder) error
