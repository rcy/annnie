package aqi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"goirc/internal/responder"
)

type iqairErrorResponse struct {
	Status string `json:"status"`
	Data   struct {
		Message string `json:"message"`
	} `json:"data"`
}

type iqairResponse struct {
	Status string `json:"status"`
	Data   struct {
		City    string `json:"city"`
		State   string `json:"state"`
		Country string `json:"country"`
		Current struct {
			Pollution struct {
				Ts     string `json:"ts"`
				Aqius  int    `json:"aqius"`
				Mainus string `json:"mainus"`
				Aqicn  int    `json:"aqicn"`
				Maincn string `json:"maincn"`
			} `json:"pollution"`
			Weather struct {
				Tp int     `json:"tp"`
				Pr int     `json:"pr"`
				Hu int     `json:"hu"`
				Ws float64 `json:"ws"`
			} `json:"weather"`
		} `json:"current"`
	} `json:"data"`
}

type geocodeResult struct {
	Name    string  `json:"name"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Country string  `json:"country"`
	State   string  `json:"state"`
}

func (r iqairResponse) String() string {
	if r.Status != "success" {
		return "could not retrieve AQI data"
	}

	p := r.Data.Current.Pollution
	level := aqiLevel(p.Aqius)

	parts := []string{
		fmt.Sprintf("%s, %s, %s", r.Data.City, r.Data.State, r.Data.Country),
		fmt.Sprintf("AQI %d %s %s", p.Aqius, level.emoji, level.label),
	}

	w := r.Data.Current.Weather
	parts = append(parts, fmt.Sprintf("%d°C, %d%% humidity, wind %.1f m/s", w.Tp, w.Hu, w.Ws))

	return strings.Join(parts, " — ")
}

type aqiLevelInfo struct {
	emoji string
	label string
}

func aqiLevel(aqi int) aqiLevelInfo {
	switch {
	case aqi <= 50:
		return aqiLevelInfo{"🟢", "Good"}
	case aqi <= 100:
		return aqiLevelInfo{"🟡", "Moderate"}
	case aqi <= 150:
		return aqiLevelInfo{"🟠", "Unhealthy for Sensitive Groups"}
	case aqi <= 200:
		return aqiLevelInfo{"🔴", "Unhealthy"}
	case aqi <= 300:
		return aqiLevelInfo{"🟣", "Very Unhealthy"}
	default:
		return aqiLevelInfo{"🟤", "Hazardous"}
	}
}

func geocodeCity(city string) (*geocodeResult, error) {
	key := os.Getenv("OPENWEATHERMAP_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("OPENWEATHERMAP_API_KEY not set")
	}

	u, err := url.Parse("http://api.openweathermap.org/geo/1.0/direct")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("q", city)
	q.Set("limit", "1")
	q.Set("appid", key)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []geocodeResult
	err = json.NewDecoder(resp.Body).Decode(&results)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("city not found: %s", city)
	}

	return &results[0], nil
}

func fetchAQIByCoords(lat, lon float64) (*iqairResponse, error) {
	key := os.Getenv("IQAIR_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("IQAIR_API_KEY not set")
	}

	u, err := url.Parse("http://api.airvisual.com/v2/nearest_city")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("lat", fmt.Sprintf("%f", lat))
	q.Set("lon", fmt.Sprintf("%f", lon))
	q.Set("key", key)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var r iqairResponse
	err = json.Unmarshal(body, &r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if r.Status != "success" {
		var errResp iqairErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Data.Message != "" {
			return nil, fmt.Errorf("API error: %s", errResp.Data.Message)
		}
		return nil, fmt.Errorf("API error")
	}

	return &r, nil
}

func fetchAQIByCity(city, state, country string) (*iqairResponse, error) {
	key := os.Getenv("IQAIR_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("IQAIR_API_KEY not set")
	}

	u, err := url.Parse("http://api.airvisual.com/v2/city")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("key", key)
	q.Set("city", city)
	q.Set("state", state)
	q.Set("country", country)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var r iqairResponse
	err = json.Unmarshal(body, &r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if r.Status != "success" {
		var errResp iqairErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Data.Message != "" {
			return nil, fmt.Errorf("API error: %s", errResp.Data.Message)
		}
		return nil, fmt.Errorf("API error for %s", city)
	}

	return &r, nil
}

var countryCodes = map[string]string{
	"canada":     "CA",
	"usa":        "US",
	"united states": "US",
	"america":    "US",
	"mexico":     "MX",
	"uk":         "GB",
	"united kingdom": "GB",
	"england":    "GB",
	"germany":    "DE",
	"france":     "FR",
	"italy":      "IT",
	"spain":      "ES",
	"japan":      "JP",
	"china":      "CN",
	"australia":  "AU",
	"brazil":     "BR",
	"india":      "IN",
	"russia":     "RU",
	"south korea": "KR",
	"korea":      "KR",
}

func resolveCountry(country string) string {
	if code, ok := countryCodes[strings.ToLower(country)]; ok {
		return code
	}
	return country
}

func fetchAQI(rawQuery string) (*iqairResponse, error) {
	city, state, country := parseQuery(rawQuery)

	if state != "" && country != "" {
		return fetchAQIByCity(city, state, country)
	}

	if country != "" {
		result, err := fetchAQIByCity(city, "", country)
		if err == nil {
			return result, nil
		}
		geo, err := geocodeCity(city + ",," + resolveCountry(country))
		if err == nil {
			return fetchAQIByCoords(geo.Lat, geo.Lon)
		}
	}

	geo, err := geocodeCity(city)
	if err != nil {
		return nil, err
	}

	return fetchAQIByCoords(geo.Lat, geo.Lon)
}

func parseQuery(q string) (city, state, country string) {
	parts := strings.Split(q, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	switch len(parts) {
	case 0:
		return "", "", ""
	case 1:
		return parts[0], "", ""
	case 2:
		return parts[0], "", parts[1]
	default:
		return parts[0], parts[1], parts[2]
	}
}

func Handle(params responder.Responder) error {
	q := ""
	if len(params.Matches()) > 1 {
		q = params.Match(1)
	}
	if q == "" {
		return fmt.Errorf("usage: !aqi <city>")
	}

	result, err := fetchAQI(q)
	if err != nil {
		return err
	}

	params.Privmsgf(params.Target(), "%s", result.String())
	return nil
}
