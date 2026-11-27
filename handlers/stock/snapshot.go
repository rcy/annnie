package stock

import (
	"context"
	"errors"
	"fmt"
	"goirc/db/model"
	db "goirc/model"
	"log"
	"sync"
)

// SnapshotPortfolios computes the current portfolio value for every nick that
// has stock transactions and stores them under the given date (YYYY-MM-DD).
//
// It is all or nothing: every price is fetched before anything is written, and
// if any fetch fails it returns an error without writing, so the caller can
// retry. All snapshots are then written in one transaction. Writes are upserts
// on (date, nick), so retrying is safe.
func SnapshotPortfolios(ctx context.Context, date string) error {
	q := model.New(db.DB.DB)

	nicks, err := q.DistinctStockNicks(ctx)
	if err != nil {
		return fmt.Errorf("stock snapshot: list nicks: %w", err)
	}

	// Load every nick's cash and holdings, and collect the symbols to price.
	type portfolio struct {
		nick     string
		cash     float64
		holdings map[string]float64
	}
	portfolios := make([]portfolio, 0, len(nicks))
	symbols := map[string]bool{}
	for _, nick := range nicks {
		txns, err := q.NickStockTransactions(ctx, nick)
		if err != nil {
			return fmt.Errorf("stock snapshot: %s transactions: %w", nick, err)
		}
		holdings := map[string]float64{}
		for _, t := range txns {
			switch t.Verb {
			case "BUY":
				holdings[t.Symbol] += t.Shares
			case "SELL":
				holdings[t.Symbol] -= t.Shares
			}
		}
		for sym, shares := range holdings {
			if shares < 0.0001 {
				delete(holdings, sym)
				continue
			}
			symbols[sym] = true
		}
		portfolios = append(portfolios, portfolio{nick: nick, cash: cashBalance(txns), holdings: holdings})
	}

	prices, err := fetchPrices(symbols)
	if err != nil {
		return fmt.Errorf("stock snapshot: %w", err)
	}

	// Every price is in hand; write all snapshots together.
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("stock snapshot: begin: %w", err)
	}
	defer tx.Rollback()
	qtx := q.WithTx(tx)

	for _, p := range portfolios {
		value := p.cash
		for sym, shares := range p.holdings {
			value += shares * prices[sym]
		}
		err := qtx.UpsertPortfolioSnapshot(ctx, model.UpsertPortfolioSnapshotParams{
			Date:  date,
			Nick:  p.nick,
			Value: value,
		})
		if err != nil {
			return fmt.Errorf("stock snapshot: %s upsert: %w", p.nick, err)
		}
		log.Printf("stock snapshot: %s %s = $%.2f", date, p.nick, value)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("stock snapshot: commit: %w", err)
	}
	return nil
}

// fetchPrices fetches the current price of each symbol in parallel. It fails
// if any symbol can't be priced.
func fetchPrices(symbols map[string]bool) (map[string]float64, error) {
	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		prices = make(map[string]float64, len(symbols))
		errs   []error
	)
	for sym := range symbols {
		wg.Add(1)
		go func() {
			defer wg.Done()
			price, err := fetchPrice(sym)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			prices[sym] = price
		}()
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return prices, nil
}

func fetchPrice(sym string) (float64, error) {
	qr, err := fetchQuote(sym, "1d", "1d")
	if err != nil {
		return 0, fmt.Errorf("fetch %s: %w", sym, err)
	}
	if len(qr.Chart.Result) == 0 {
		return 0, fmt.Errorf("%s not found", sym)
	}
	return qr.Chart.Result[0].Meta.RegularMarketPrice, nil
}
