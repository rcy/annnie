package stock

import (
	"context"
	"fmt"
	"goirc/db/model"
	"math"
	"goirc/internal/responder"
	db "goirc/model"
	"strconv"
	"strings"
	"sync"
	"time"
)

const startingCash = 1000.0

func HandleBuy(params responder.Responder) error {
	symbol := strings.ToUpper(params.Match(1))
	amountStr := params.Match(2)
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return fmt.Errorf("invalid amount: %s", amountStr)
	}

	resolved, err := resolveSymbol(symbol)
	if err != nil {
		return err
	}
	symbol = resolved

	qr, err := fetchQuote(symbol, "1d", "1d")
	if err != nil {
		return err
	}
	if qr.Chart.Error != nil {
		return fmt.Errorf("%s: %s", symbol, qr.Chart.Error.Description)
	}
	if len(qr.Chart.Result) == 0 {
		return fmt.Errorf("%s: not found", symbol)
	}
	price := qr.Chart.Result[0].Meta.RegularMarketPrice

	q := model.New(db.DB.DB)
	nick := params.Nick()
	ctx := context.Background()

	txns, err := q.NickStockTransactions(ctx, nick)
	if err != nil {
		return err
	}

	cash := cashBalance(txns)
	if cash < amount {
		params.Privmsgf(params.Target(), "%s: not enough cash ($%.2f available)", nick, cash)
		return nil
	}

	shares := amount / price

	err = q.InsertStockTransaction(ctx, model.InsertStockTransactionParams{
		Nick:      nick,
		Verb:      "BUY",
		Symbol:    symbol,
		AmountUsd: amount,
		Shares:    shares,
		Price:     price,
	})
	if err != nil {
		return err
	}

	params.Privmsgf(params.Target(), "%s: bought $%.0f of %s @ $%.2f/share", nick, amount, symbol, price)
	return nil
}

func HandleSell(params responder.Responder) error {
	symbol := strings.ToUpper(params.Match(1))
	amountStr := params.Match(2)
	sellAll := amountStr == ""

	resolved, err := resolveSymbol(symbol)
	if err != nil {
		return err
	}
	symbol = resolved

	qr, err := fetchQuote(symbol, "1d", "1d")
	if err != nil {
		return err
	}
	if qr.Chart.Error != nil {
		return fmt.Errorf("%s: %s", symbol, qr.Chart.Error.Description)
	}
	if len(qr.Chart.Result) == 0 {
		return fmt.Errorf("%s: not found", symbol)
	}
	price := qr.Chart.Result[0].Meta.RegularMarketPrice

	q := model.New(db.DB.DB)
	nick := params.Nick()
	ctx := context.Background()

	txns, err := q.NickStockTransactions(ctx, nick)
	if err != nil {
		return err
	}

	held := sharesHeld(txns, symbol)

	var amount, shares float64
	if sellAll {
		if held < 0.0001 {
			params.Privmsgf(params.Target(), "%s: you don't hold any %s", nick, symbol)
			return nil
		}
		shares = held
		amount = shares * price
	} else {
		amount, err = strconv.ParseFloat(amountStr, 64)
		if err != nil {
			return fmt.Errorf("invalid amount: %s", amountStr)
		}
		shares = amount / price
		currentValue := held * price
		if currentValue < amount {
			params.Privmsgf(params.Target(), "%s: not enough %s ($%.2f worth held)", nick, symbol, currentValue)
			return nil
		}
	}

	err = q.InsertStockTransaction(ctx, model.InsertStockTransactionParams{
		Nick:      nick,
		Verb:      "SELL",
		Symbol:    symbol,
		AmountUsd: amount,
		Shares:    shares,
		Price:     price,
	})
	if err != nil {
		return err
	}

	params.Privmsgf(params.Target(), "%s: sold $%.2f of %s @ $%.2f/share", nick, amount, symbol, price)
	return nil
}

// lot represents a single buy that may have been partially sold (FIFO).
type lot struct {
	symbol    string
	buyDate   time.Time
	buyPrice  float64
	shares    float64 // remaining shares in this lot
	costBasis float64 // remaining cost basis for this lot
}

// buildLots computes open lots from a transaction log using FIFO order.
func buildLots(txns []model.StockTransaction) []lot {
	var lots []lot
	for _, t := range txns {
		switch t.Verb {
		case "BUY":
			lots = append(lots, lot{
				symbol:    t.Symbol,
				buyDate:   t.CreatedAt,
				buyPrice:  t.Price,
				shares:    t.Shares,
				costBasis: t.AmountUsd,
			})
		case "SELL":
			remaining := t.Shares
			for i := range lots {
				if lots[i].symbol != t.Symbol || lots[i].shares < 0.0001 {
					continue
				}
				if lots[i].shares <= remaining {
					remaining -= lots[i].shares
					lots[i].costBasis = 0
					lots[i].shares = 0
				} else {
					lots[i].costBasis -= (remaining / lots[i].shares) * lots[i].costBasis
					lots[i].shares -= remaining
					remaining = 0
				}
				if remaining < 0.0001 {
					break
				}
			}
		}
	}
	// Filter out empty lots
	var open []lot
	for _, l := range lots {
		if l.shares >= 0.0001 {
			open = append(open, l)
		}
	}
	return open
}

func HandlePortfolio(params responder.Responder) error {
	q := model.New(db.DB.DB)
	nick := params.Nick()
	ctx := context.Background()

	txns, err := q.NickStockTransactions(ctx, nick)
	if err != nil {
		return err
	}

	cash := cashBalance(txns)
	lots := buildLots(txns)

	if len(lots) == 0 {
		params.Privmsgf(params.Target(), "%s: cash $%.0f | no holdings | total $%.0f", nick, math.Floor(cash), math.Floor(cash))
		return nil
	}

	// Collect unique symbols to fetch prices
	symbolSet := map[string]bool{}
	for _, l := range lots {
		symbolSet[l.symbol] = true
	}

	// Fetch current prices in parallel
	type priceResult struct {
		symbol string
		price  float64
		err    error
	}
	var wg sync.WaitGroup
	priceCh := make(chan priceResult, len(symbolSet))
	for sym := range symbolSet {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			qr, err := fetchQuote(s, "1d", "1d")
			if err != nil {
				priceCh <- priceResult{s, 0, err}
				return
			}
			if len(qr.Chart.Result) == 0 {
				priceCh <- priceResult{s, 0, fmt.Errorf("%s: not found", s)}
				return
			}
			priceCh <- priceResult{s, qr.Chart.Result[0].Meta.RegularMarketPrice, nil}
		}(sym)
	}
	wg.Wait()
	close(priceCh)

	prices := map[string]float64{}
	for r := range priceCh {
		if r.err != nil {
			return r.err
		}
		prices[r.symbol] = r.price
	}

	// Build output — single line
	var totalValue float64
	var holdings []string
	for _, l := range lots {
		mktVal := l.shares * prices[l.symbol]
		totalValue += mktVal
		pnl := mktVal - l.costBasis
		arrow := "↑"
		if pnl < 0 {
			arrow = "↓"
			pnl = -pnl
		}
		holdings = append(holdings, fmt.Sprintf("%s %s $%.0f%s%.0f", l.symbol, l.buyDate.Format("Jan2"), mktVal, arrow, pnl))
	}

	portfolio := cash + totalValue
	pnl := portfolio - startingCash
	arrow := "↑"
	if pnl < 0 {
		arrow = "↓"
		pnl = -pnl
	}
	parts := []string{fmt.Sprintf("total $%.0f%s%.0f", portfolio, arrow, pnl), fmt.Sprintf("cash $%.0f", math.Floor(cash))}
	parts = append(parts, holdings...)
	params.Privmsgf(params.Target(), "%s: %s", nick, strings.Join(parts, ", "))
	return nil
}

func cashBalance(txns []model.StockTransaction) float64 {
	cash := startingCash
	for _, t := range txns {
		switch t.Verb {
		case "BUY":
			cash -= t.AmountUsd
		case "SELL", "CREDIT":
			cash += t.AmountUsd
		}
	}
	return cash
}

func Credit(ctx context.Context, nick string, amount float64) error {
	q := model.New(db.DB)
	return q.InsertStockTransaction(ctx, model.InsertStockTransactionParams{
		Nick:      nick,
		Verb:      "CREDIT",
		Symbol:    "",
		AmountUsd: amount,
		Shares:    0,
		Price:     0,
	})
}

func sharesHeld(txns []model.StockTransaction, symbol string) float64 {
	var shares float64
	for _, t := range txns {
		if t.Symbol != symbol {
			continue
		}
		switch t.Verb {
		case "BUY":
			shares += t.Shares
		case "SELL":
			shares -= t.Shares
		}
	}
	return shares
}
