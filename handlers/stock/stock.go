package stock

import (
	"encoding/json"
	"fmt"
	"goirc/internal/responder"
	"net/http"
	"strings"
)

type quoteResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Symbol             string  `json:"symbol"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				ChartPreviousClose float64 `json:"chartPreviousClose"`
				Currency           string  `json:"currency"`
				ShortName          string  `json:"shortName"`
			} `json:"meta"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

func fetchQuote(symbol string) (*quoteResponse, error) {
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=1d&interval=1d", symbol)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var qr quoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&qr); err != nil {
		return nil, err
	}

	return &qr, nil
}

func Handle(params responder.Responder) error {
	symbol := strings.ToUpper(params.Match(1))

	qr, err := fetchQuote(symbol)
	if err != nil {
		return err
	}

	if qr.Chart.Error != nil {
		return fmt.Errorf("%s: %s", symbol, qr.Chart.Error.Description)
	}

	if len(qr.Chart.Result) == 0 {
		return fmt.Errorf("%s: not found", symbol)
	}

	meta := qr.Chart.Result[0].Meta
	price := meta.RegularMarketPrice
	prevClose := meta.ChartPreviousClose
	currency := meta.Currency

	change := price - prevClose
	changePct := 0.0
	if prevClose > 0 {
		changePct = (change / prevClose) * 100
	}

	sign := ""
	if change > 0 {
		sign = "+"
	}

	params.Privmsgf(params.Target(), "%s: %.2f %s (%s%.2f / %s%.2f%%)",
		symbol, price, currency, sign, change, sign, changePct)

	return nil
}
