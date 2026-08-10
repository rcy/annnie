package weather

import (
	"context"
	"encoding/json"
	"fmt"
	db "goirc/db/model"
	"goirc/handlers/aqi"
	"goirc/internal/responder"
	"goirc/model"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pariz/gountries"
)

type weather struct {
	Coord struct {
		Lon float64 `json:"lon"`
		Lat float64 `json:"lat"`
	} `json:"coord"`
	Weather []struct {
		ID          int    `json:"id"`
		Main        string `json:"main"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	} `json:"weather"`
	Base string `json:"base"`
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		TempMin   float64 `json:"temp_min"`
		TempMax   float64 `json:"temp_max"`
		Pressure  int     `json:"pressure"`
		Humidity  int     `json:"humidity"`
		SeaLevel  int     `json:"sea_level"`
		GrndLevel int     `json:"grnd_level"`
	} `json:"main"`
	Visibility int `json:"visibility"`
	Wind       struct {
		Speed float64 `json:"speed"`
		Deg   uint    `json:"deg"`
		Gust  float64 `json:"gust"`
	} `json:"wind"`
	Clouds struct {
		All int `json:"all"`
	} `json:"clouds"`
	Rain struct {
		OneH   float64 `json:"1h"`
		ThreeH float64 `json:"3h"`
	} `json:"rain"`
	Snow struct {
		OneH   float64 `json:"1h"`
		ThreeH float64 `json:"3h"`
	} `json:"snow"`
	Dt  int `json:"dt"`
	Sys struct {
		Type    int    `json:"type"`
		ID      int    `json:"id"`
		Message int    `json:"message"`
		Country string `json:"country"`
		Sunrise int    `json:"sunrise"`
		Sunset  int    `json:"sunset"`
	} `json:"sys"`
	Timezone int    `json:"timezone"`
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Cod      int    `json:"cod"`
}

func (w weather) String() string {
	components := []string{}

	temp := fmt.Sprintf("%.1f°C", w.Main.Temp)
	if w.Main.FeelsLike != w.Main.Temp {
		temp += fmt.Sprintf(" (feels like %.1f°C)", w.Main.FeelsLike)
	}
	components = append(components, temp)

	descs := []string{}
	for _, w := range w.Weather {
		descs = append(descs, w.Description)
	}
	components = append(components, strings.Join(descs, ", "))

	var snow string
	if w.Snow.ThreeH > 0 {
		snow = fmt.Sprintf("%.1fmm pcpn over last 3 hours", w.Snow.ThreeH)
	} else if w.Snow.OneH > 0 {
		snow = fmt.Sprintf("%.1fmm pcpn over last hour", w.Snow.OneH)
	}
	if snow != "" {
		components = append(components, snow)
	}

	var rain string
	if w.Rain.ThreeH > 0 {
		rain = fmt.Sprintf("%.1fmm pcpn over last 3 hours", w.Rain.ThreeH)
	} else if w.Rain.OneH > 0 {
		rain = fmt.Sprintf("%.1fmm pcpn over last hour", w.Rain.OneH)
	}
	if rain != "" {
		components = append(components, rain)
	}

	if w.Visibility > 0 && w.Visibility < 10000 {
		components = append(components, fmt.Sprintf("visibility %.1fkm", float64(w.Visibility)/1000))
	}

	if w.Wind.Speed > 0 {
		wind := fmt.Sprintf("wind %.0f kph %s", w.Wind.Speed*3.6, compass16(w.Wind.Deg))
		if w.Wind.Gust > 0 {
			wind += fmt.Sprintf(" (gust %.0f kph)", w.Wind.Gust*3.6)
		}
		components = append(components, wind)
	}

	return strings.Join(components, ", ")
}

func makeWeatherAPIURL(key string, city string) (string, error) {
	u, err := url.Parse("http://api.openweathermap.org/data/2.5/weather")
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("appid", key)
	q.Set("units", "metric")
	q.Set("q", city)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func fetchWeather(city string) (*weather, error) {
	key := os.Getenv("OPENWEATHERMAP_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("bad api key")
	}

	url, err := makeWeatherAPIURL(key, city)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("city not found")
	}

	var w weather
	err = json.NewDecoder(resp.Body).Decode(&w)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func fetchXWeather(city string) ([]byte, error) {
	key := os.Getenv("OPENWEATHERMAP_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("bad api key")
	}

	url, err := makeWeatherAPIURL(key, city)
	if err != nil {
		return nil, err
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("city not found")
	}

	return io.ReadAll(resp.Body)
}

func Handle(params responder.Responder) error {
	ctx := context.TODO()
	queries := db.New(model.DB)

	var q string
	if len(params.Matches()) > 1 {
		q = params.Match(1)
	}

	q, err := weatherQueryByNick(ctx, q, params.Nick())
	if err != nil {
		return err
	}

	weath, err := fetchWeather(q)
	if err != nil {
		return err
	}

	var countryStr string
	country, err := gountries.New().FindCountryByAlpha(weath.Sys.Country)
	if err != nil {
		countryStr = "??"
	} else {
		countryStr = country.Name.Common
	}

	err = queries.InsertNickWeatherRequest(ctx, db.InsertNickWeatherRequestParams{
		Nick:    params.Nick(),
		Query:   q,
		City:    weath.Name,
		Country: weath.Sys.Country,
	})
	if err != nil {
		return err
	}

	weatherStr := weath.String()

	airQuality, err := aqi.FetchAQIByCoords(weath.Coord.Lat, weath.Coord.Lon)
	if err == nil {
		if aqiStr := airQuality.AQIString(); aqiStr != "" {
			weatherStr += ", " + aqiStr
		}
	}

	params.Privmsgf(params.Target(), "%s, %s today: %s", weath.Name, countryStr, weatherStr)

	return nil
}

func XHandle(params responder.Responder) error {
	q := params.Match(1)

	resp, err := fetchXWeather(q)
	if err != nil {
		return err
	}

	chunks := splitBytes(resp, 420)

	for _, chunk := range chunks {
		params.Privmsgf(params.Target(), "%s", string(chunk))
	}

	return nil
}

func splitBytes(data []byte, chunkSize int) [][]byte {
	var chunks [][]byte

	for len(data) > 0 {
		if len(data) < chunkSize {
			chunkSize = len(data)
		}
		chunks = append(chunks, data[:chunkSize])
		data = data[chunkSize:]
	}

	return chunks
}
