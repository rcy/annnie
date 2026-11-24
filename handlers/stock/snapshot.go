package stock

import (
	"context"
	"goirc/db/model"
	db "goirc/model"
	"log"
	"time"
)

// SnapshotPortfolios computes and stores the current portfolio value for every
// nick that has stock transactions. Intended to be called once daily.
func SnapshotPortfolios() {
	ctx := context.Background()
	q := model.New(db.DB.DB)

	nicks, err := q.DistinctStockNicks(ctx)
	if err != nil {
		log.Printf("stock snapshot: list nicks: %v", err)
		return
	}

	date := time.Now().Format("2006-01-02")

	for _, nick := range nicks {
		txns, err := q.NickStockTransactions(ctx, nick)
		if err != nil {
			log.Printf("stock snapshot: %s: %v", nick, err)
			continue
		}

		cash := cashBalance(txns)

		// Compute shares held per symbol
		holdings := map[string]float64{}
		for _, t := range txns {
			switch t.Verb {
			case "BUY":
				holdings[t.Symbol] += t.Shares
			case "SELL":
				holdings[t.Symbol] -= t.Shares
			}
		}

		// Fetch current prices
		totalValue := cash
		for sym, shares := range holdings {
			if shares < 0.0001 {
				continue
			}
			qr, err := fetchQuote(sym, "1d", "1d")
			if err != nil {
				log.Printf("stock snapshot: %s fetch %s: %v", nick, sym, err)
				continue
			}
			if len(qr.Chart.Result) == 0 {
				log.Printf("stock snapshot: %s: %s not found", nick, sym)
				continue
			}
			price := qr.Chart.Result[0].Meta.RegularMarketPrice
			totalValue += shares * price
		}

		err = q.UpsertPortfolioSnapshot(ctx, model.UpsertPortfolioSnapshotParams{
			Date:  date,
			Nick:  nick,
			Value: totalValue,
		})
		if err != nil {
			log.Printf("stock snapshot: %s upsert: %v", nick, err)
		}
		log.Printf("stock snapshot: %s = $%.2f", nick, totalValue)
	}
}
