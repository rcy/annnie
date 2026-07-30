package ai

import (
	"context"
	"errors"
	"goirc/db/model"
	db "goirc/model"
	"strings"

	"github.com/openai/openai-go/v3"
)

var ErrBilling = errors.New("I need money: https://rcy.sh/fundannie")
var ErrRejected = errors.New("Rejected")
var Complete = CompleteDeepSeek

// isBillingError checks if an error indicates an account billing issue
// (insufficient funds, payment required, etc.)
func isBillingError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "billing") ||
		strings.Contains(msg, "Insufficient Balance") ||
		strings.Contains(msg, "insufficient_quota")
}

type diagFuncKey struct{}

// WithDiagFunc returns a context that carries fn, which will be called with
// any reasoning_content returned by the model.
func WithDiagFunc(ctx context.Context, fn func(string)) context.Context {
	return context.WithValue(ctx, diagFuncKey{}, fn)
}

func diagFuncFromContext(ctx context.Context) func(string) {
	fn, _ := ctx.Value(diagFuncKey{}).(func(string))
	return fn
}

func getModel(ctx context.Context) string {
	q := model.New(db.DB.DB)
	cfg, err := q.GetConfig(ctx, "model")
	if err != nil || cfg.Value == "" {
		return string(openai.ChatModelGPT5_4Mini)
	}
	return cfg.Value
}
