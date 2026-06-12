package ai

import (
	"context"
	"errors"

	"goirc/configs"

	"github.com/openai/openai-go/v3"
)

var ErrBilling = errors.New("I need money: https://rcy.sh/fundannie")
var ErrRejected = errors.New("Rejected")
var Complete = CompleteDeepSeek

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
	value, err := configs.Get("model")
	if err != nil || value == "" {
		return string(openai.ChatModelGPT5_4Mini)
	}
	return value
}
