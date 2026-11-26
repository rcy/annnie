package stock

import (
	"goirc/db/model"
	"testing"
)

func TestGrantsAreNotMarketGains(t *testing.T) {
	txns := []model.StockTransaction{
		{Verb: "CREDIT", AmountUsd: 40},
		{Verb: "BUY", Symbol: "X", AmountUsd: 40, Shares: 4, Price: 10},
	}

	cash := cashBalance(txns)
	if cash != 1000 {
		t.Errorf("cash: got %v, want 1000", cash)
	}

	// the 4 shares are now worth $50
	if got := marketPnL(cash+50, txns); got != 10 {
		t.Errorf("pnl: got %v, want 10", got)
	}
}
