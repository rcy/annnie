package lua

import (
	"errors"
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

func withState(t *testing.T, code string) func() {
	t.Helper()

	mu.Lock()
	defer mu.Unlock()

	state = lua.NewState()
	if err := loadState(state, code); err != nil {
		t.Fatalf("loadState: %v", err)
	}

	return func() {
		mu.Lock()
		defer mu.Unlock()
		state.Close()
		state = nil
		registeredCommands = map[string]*luaCommand{}
	}
}

func TestRegisterCommandInvoke(t *testing.T) {
	cleanup := withState(t, `
		function greet(name)
			return "hello " .. name
		end
		register_command("greet", greet, "greets someone")
	`)
	defer cleanup()

	got, err := Invoke("greet", "world")
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if got != "hello world" {
		t.Fatalf("expected 'hello world', got %q", got)
	}
}

func TestInvokePrintsToOut(t *testing.T) {
	cleanup := withState(t, `
		register_command("say", function()
			print("printable output")
		end, "prints output")
	`)
	defer cleanup()

	got, err := Invoke("say", "")
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if got != "printable output" {
		t.Fatalf("expected 'printable output', got %q", got)
	}
}

func TestInvokeUnknownCommand(t *testing.T) {
	cleanup := withState(t, "")
	defer cleanup()

	_, err := Invoke("nope", "")
	if !errors.Is(err, ErrCommandNotFound) {
		t.Fatalf("expected ErrCommandNotFound, got %v", err)
	}
}

func TestListCommands(t *testing.T) {
	cleanup := withState(t, `
		register_command("zebra", function() return 1 end, "z command")
		register_command("alpha", function() return 2 end, "a command")
		register_command("nodoc", function() return 3 end)
	`)
	defer cleanup()

	cmds := ListCommands()
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}
	if cmds[0].Name != "alpha" || cmds[0].Desc != "a command" {
		t.Fatalf("expected alpha first with 'a command', got %+v", cmds[0])
	}
	if cmds[1].Name != "nodoc" || cmds[1].Desc != "" {
		t.Fatalf("expected nodoc second with no desc, got %+v", cmds[1])
	}
	if cmds[2].Name != "zebra" {
		t.Fatalf("expected zebra last, got %+v", cmds[2])
	}
}

func TestInvalidCommandName(t *testing.T) {
	mu.Lock()
	L := lua.NewState()
	err := loadState(L, `register_command("bad name", function() return 1 end)`)
	L.Close()
	registeredCommands = map[string]*luaCommand{}
	mu.Unlock()

	if err == nil {
		t.Fatal("expected error for invalid command name")
	}
}

func TestRegistryClearedOnReload(t *testing.T) {
	mu.Lock()
	defer mu.Unlock()

	L := lua.NewState()
	defer L.Close()

	if err := loadState(L, `register_command("old", function() return 1 end)`); err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if _, ok := registeredCommands["old"]; !ok {
		t.Fatal("expected 'old' to be registered")
	}

	if err := loadState(L, `register_command("new", function() return 2 end)`); err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if _, ok := registeredCommands["old"]; ok {
		t.Fatal("expected 'old' to be cleared on reload")
	}
	if _, ok := registeredCommands["new"]; !ok {
		t.Fatal("expected 'new' to be registered after reload")
	}
}
