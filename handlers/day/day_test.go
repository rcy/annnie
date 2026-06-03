package day

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func makeServer(t *testing.T, html string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
}

func makeFileServer(t *testing.T, path string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("reading %s: %v", path, err)
			http.Error(w, "not found", 500)
			return
		}
		w.Write(b)
	}))
}

func TestFetchDayEvents(t *testing.T) {
	tests := []struct {
		name string
		html string
		want []string
	}{
		{
			name: "extracts matching card titles",
			html: `<html><body>
				<h2 class="card__title"><a class="js-link-target">National Cat Day</a></h2>
				<h2 class="card__title"><a class="js-link-target">National Dog Day</a></h2>
				<h2 class="card__title"><a class="js-link-target">Some Unrelated Event</a></h2>
			</body></html>`,
			want: []string{"National Cat Day", "National Dog Day"},
		},
		{
			name: "no matches returns empty stack",
			html: `<html><body><h2 class="card__title"><a class="js-link-target">Cheese Festival</a></h2></body></html>`,
			want: []string{},
		},
		{
			name: "case insensitive — lowercase day matched",
			html: `<html><body><h2 class="card__title"><a class="js-link-target">national cat day</a></h2></body></html>`,
			want: []string{"national cat day"},
		},
		{
			name: "link outside article not matched",
			html: `<html><body><div><a class="js-link-target">National Soup Day</a></div></body></html>`,
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := makeServer(t, tt.html)
			defer srv.Close()

			// Patch fetchDayEvents to use the test server URL instead of daysoftheyear.com.
			origFetch := fetchURL
			fetchURL = func(day string) string { return srv.URL }
			defer func() { fetchURL = origFetch }()

			s, err := fetchDayEvents("jan/01")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var got []string
			for {
				item, ok := s.Pop()
				if !ok {
					break
				}
				got = append(got, item)
			}
			if got == nil {
				got = []string{}
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("item %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFetchDayEventsJun02(t *testing.T) {
	srv := makeFileServer(t, "testdata/jun02.html")
	defer srv.Close()

	origFetch := fetchURL
	fetchURL = func(day string) string { return srv.URL }
	defer func() { fetchURL = origFetch }()

	s, err := fetchDayEvents("jun/02")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"National I Love My Dentist Day",
		"Republic Day Italy",
		"National Rotisserie Chicken Day",
		"National Rocky Road Day",
		"National Greyhound Day",
		"National Leave The Office Earlier Day",
		"International Volkswagen Bus Day",
		"National Bubba Day",
		"American Indian Citizenship Day",
	}
	var got []string
	for {
		item, ok := s.Pop()
		if !ok {
			break
		}
		got = append(got, item)
	}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
