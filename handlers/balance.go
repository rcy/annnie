package handlers

import (
	"context"
	"fmt"
	"goirc/internal/ai"
	"goirc/internal/responder"
	"strings"
)

func Balance(params responder.Responder) error {
	balance, err := ai.GetDeepSeekBalance(context.TODO())
	if err != nil {
		return fmt.Errorf("GetDeepSeekBalance: %w", err)
	}

	parts := make([]string, 0, len(balance.BalanceInfos))
	for _, info := range balance.BalanceInfos {
		parts = append(parts, fmt.Sprintf("%s: %s (granted: %s, topped up: %s)",
			info.Currency, info.TotalBalance, info.GrantedBalance, info.ToppedUpBalance))
	}

	available := "unavailable"
	if balance.IsAvailable {
		available = "available"
	}

	params.Privmsgf(params.Target(), "%s: deepseek balance (%s): %s",
		params.Nick(), available, strings.Join(parts, " | "))
	return nil
}
