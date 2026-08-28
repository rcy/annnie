package util

import (
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/rcy/durfmt"
)

func ParseTime(str string) (time.Time, error) {
	result, err := time.Parse("2006-01-02 15:04:05", str)
	if err != nil {
		result, err = time.Parse("2006-01-02T15:04:05Z", str)
	}
	return result, err
}

func Since(tstr string) string {
	t, err := ParseTime(tstr)
	if err != nil {
		log.Fatal(err)
	}
	return Ago(time.Since(t).Round(time.Second))
}

func Ago(d time.Duration) string {
	return durfmt.Format(d)
}

// from a uri like https://www.google.com/abc?def=123 return google.com
func BareDomain(uri string) string {
	parsedUrl, err := url.Parse(uri)
	if err != nil {
		// just punt and return the original uri
		return uri
	}
	return strings.Replace(parsedUrl.Host, "www.", "", 1)
}


