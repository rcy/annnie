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
				LongName           string  `json:"longName"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				ChartPreviousClose float64 `json:"chartPreviousClose"`
				Currency           string  `json:"currency"`
			} `json:"meta"`
			Indicators struct {
				Quote []struct {
					Close []interface{} `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

func fetchQuote(symbol, rangePeriod, interval string) (*quoteResponse, error) {
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=%s&interval=%s", symbol, rangePeriod, interval)

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

func firstClose(qr *quoteResponse) (float64, bool) {
	if len(qr.Chart.Result) == 0 {
		return 0, false
	}
	closes := qr.Chart.Result[0].Indicators.Quote
	if len(closes) == 0 {
		return 0, false
	}
	for _, v := range closes[0].Close {
		if f, ok := toFloat(v); ok {
			return f, true
		}
	}
	return 0, false
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case nil:
		return 0, false
	default:
		return 0, false
	}
}

type searchResponse struct {
	Quotes []struct {
		Symbol string `json:"symbol"`
	} `json:"quotes"`
}

// resolveSymbol tries the input as a ticker first; if the chart API returns
// an error, it searches Yahoo Finance and returns the top result's symbol.
func resolveSymbol(input string) (string, error) {
	// Try as a ticker first with a cheap 1d request
	qr, err := fetchQuote(input, "1d", "1d")
	if err == nil && qr.Chart.Error == nil && len(qr.Chart.Result) > 0 {
		return input, nil
	}

	// Fall back to search
	url := fmt.Sprintf("https://query2.finance.yahoo.com/v1/finance/search?q=%s&quotesCount=1&newsCount=0", input)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", err
	}

	if len(sr.Quotes) == 0 {
		return "", fmt.Errorf("%s: not found", input)
	}

	return sr.Quotes[0].Symbol, nil
}

func formatChange(price, baseline float64) string {
	if baseline == 0 {
		return "n/a"
	}
	change := price - baseline
	pct := (change / baseline) * 100
	if pct > -0.005 && pct < 0.005 {
		return ""
	}
	sign := ""
	if change > 0 {
		sign = "+"
	}
	return fmt.Sprintf("%s%.2f%%", sign, pct)
}

func Handle(params responder.Responder) error {
	input := strings.ToUpper(params.Match(1))

	symbol, err := resolveSymbol(input)
	if err != nil {
		return err
	}

	type period struct {
		label    string
		rangePd  string
		interval string
	}
	periods := []period{
		{"1d", "1d", "1d"},
		{"7d", "7d", "1d"},
		{"30d", "1mo", "1d"},
		{"1y", "1y", "1mo"},
	}

	// Fetch all periods in parallel
	type result struct {
		label string
		qr    *quoteResponse
		err   error
	}
	ch := make(chan result, len(periods))
	for _, p := range periods {
		go func(p period) {
			qr, err := fetchQuote(symbol, p.rangePd, p.interval)
			ch <- result{p.label, qr, err}
		}(p)
	}

	results := make(map[string]*quoteResponse)
	for range periods {
		r := <-ch
		if r.err != nil {
			return r.err
		}
		if r.qr.Chart.Error != nil {
			return fmt.Errorf("%s: %s", symbol, r.qr.Chart.Error.Description)
		}
		if len(r.qr.Chart.Result) == 0 {
			return fmt.Errorf("%s: not found", symbol)
		}
		results[r.label] = r.qr
	}

	// Get current price and name
	meta := results["1d"].Chart.Result[0].Meta
	price := meta.RegularMarketPrice
	currency := meta.Currency
	name := meta.LongName

	// Build change strings, omitting zero/empty changes
	changes := []string{}
	for _, p := range periods {
		qr := results[p.label]
		var baseline float64
		if b, ok := firstClose(qr); ok {
			baseline = b
		} else {
			baseline = qr.Chart.Result[0].Meta.ChartPreviousClose
		}
		if baseline == 0 {
			continue
		}
		s := formatChange(price, baseline)
		if s == "" {
			continue
		}
		changes = append(changes, fmt.Sprintf("%s: %s", p.label, s))
	}

	nameStr := ""
	if name != "" {
		nameStr = name + " "
	}

	params.Privmsgf(params.Target(), "%s: %s%.2f %s (%s)",
		symbol, nameStr, price, currency, strings.Join(changes, ", "))

	return nil
}
