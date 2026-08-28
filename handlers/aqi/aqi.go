package aqi

import (
	"encoding/json"
	"fmt"
	"goirc/config"
	"io"
	"net/http"
	"net/url"
	"strings"
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

func (r iqairResponse) AQIString() string {
	if r.Status != "success" {
		return ""
	}
	p := r.Data.Current.Pollution
	level := aqiLevel(p.Aqius)
	return fmt.Sprintf("aqi %d %s", p.Aqius, level.emoji)
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

func FetchAQIByCoords(lat, lon float64) (*iqairResponse, error) {
	key := config.Get().IQAIRAPIKey
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
