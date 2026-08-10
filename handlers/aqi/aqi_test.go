package aqi

import (
	"encoding/json"
	"os"
	"testing"
)

func TestString(t *testing.T) {
	payload, err := os.ReadFile("testdata/creston,bc,canada.json")
	if err != nil {
		t.Fatal(err)
	}

	var r iqairResponse
	err = json.Unmarshal(payload, &r)
	if err != nil {
		t.Fatal(err)
	}

	got := r.String()
	want := "Creston, British Columbia, Canada — AQI 42 🟢 Good — -5°C, 60% humidity, wind 3.5 m/s"

	if got != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", got, want)
	}
}

func TestParseQuery(t *testing.T) {
	for _, tc := range []struct {
		q                     string
		city, state, country  string
	}{
		{"creston", "creston", "", ""},
		{"creston, bc, canada", "creston", "bc", "canada"},
		{"los angeles, california, usa", "los angeles", "california", "usa"},
		{"beijing, china", "beijing", "", "china"},
		{"vancouver, bc", "vancouver", "", "bc"},
		{"", "", "", ""},
	} {
		city, state, country := parseQuery(tc.q)
		if city != tc.city || state != tc.state || country != tc.country {
			t.Errorf("parseQuery(%q): got (%q, %q, %q), want (%q, %q, %q)",
				tc.q, city, state, country, tc.city, tc.state, tc.country)
		}
	}
}

func TestAQILevel(t *testing.T) {
	for _, tc := range []struct {
		aqi   int
		label string
	}{
		{0, "Good"},
		{50, "Good"},
		{51, "Moderate"},
		{100, "Moderate"},
		{101, "Unhealthy for Sensitive Groups"},
		{150, "Unhealthy for Sensitive Groups"},
		{151, "Unhealthy"},
		{200, "Unhealthy"},
		{201, "Very Unhealthy"},
		{300, "Very Unhealthy"},
		{301, "Hazardous"},
		{500, "Hazardous"},
	} {
		got := aqiLevel(tc.aqi)
		if got.label != tc.label {
			t.Errorf("aqiLevel(%d): got label %q, want %q", tc.aqi, got.label, tc.label)
		}
	}
}
