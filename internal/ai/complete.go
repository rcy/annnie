package ai

import (
	"context"
	"errors"
	"goirc/db/model"
	db "goirc/model"

	"github.com/openai/openai-go/v3"
)

var ErrBilling = errors.New("I need money: https://rcy.sh/fundannie")
var ErrRejected = errors.New("Rejected")
var Complete = CompleteOllama

func getModel(ctx context.Context) string {
	q := model.New(db.DB.DB)
	cfg, err := q.GetConfig(ctx, "model")
	if err != nil || cfg.Value == "" {
		return string(openai.ChatModelGPT5_4Mini)
	}
	return cfg.Value
}
