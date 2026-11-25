package main

import (
	"goirc/bot"
	"strings"
	"testing"

	irc "github.com/thoj/go-ircevent"
)

func TestMain(t *testing.T) {
	var b bot.Bot
	b.Conn = irc.IRC("testbot", "testbot")

	addHandlers(&b)

	for _, tc := range []struct {
		message string
		want    []string
	}{
		{
			"69",
			[]string{"Nice"},
		},
		{
			"691",
			nil,
		},
		{
			"169",
			nil,
		},
		{
			"69.",
			[]string{"Nice"},
		},
		{
			"69th",
			[]string{"Nice"},
		},
		{
			"x69",
			nil,
		},
	} {
		t.Run(tc.message, func(t *testing.T) {
			var got []string
			for _, handler := range b.Handlers {
				matches := handler.Regexp().FindStringSubmatch(tc.message)
				if len(matches) > 0 {
					got = append(got, strings.Fields(handler.String())[1])
				}
			}
			if len(tc.want) != len(got) {
				t.Fatalf("wanted %v, got %v", tc.want, got)

			}
			for i := range tc.want {
				if tc.want[i] != got[i] {
					t.Fatalf("wanted %v, got %v", tc.want, got)
				}
			}
		})
	}
}
