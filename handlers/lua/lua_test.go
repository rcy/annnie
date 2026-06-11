package lua

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestHttpGetURLParsing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer ts.Close()

	L := lua.NewState()
	defer L.Close()

	outBuf.Reset()
	setupPrint(L)
	setupHTTP(L)

	code := `print(http.get("` + ts.URL + `"))`
	if err := L.DoString(code); err != nil {
		t.Fatalf("Lua error: %v", err)
	}

	out := strings.TrimSpace(outBuf.String())
	if out != "hello" {
		t.Fatalf("expected 'hello', got %q", out)
	}
}

func TestHttpJSONParsing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"key":"value","num":42}`))
	}))
	defer ts.Close()

	L := lua.NewState()
	defer L.Close()

	outBuf.Reset()
	setupPrint(L)
	setupHTTP(L)

	code := `local t = http.json("` + ts.URL + `"); print(t.key, t.num)`
	if err := L.DoString(code); err != nil {
		t.Fatalf("Lua error: %v", err)
	}

	out := strings.TrimSpace(outBuf.String())
	if out != "value\t42" {
		t.Fatalf("expected 'value\t42', got %q", out)
	}
}
