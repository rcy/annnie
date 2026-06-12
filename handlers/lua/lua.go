package lua

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"goirc/configs"
	"goirc/internal/responder"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

var (
	mu     sync.Mutex
	state  *lua.LState
	outBuf bytes.Buffer
)

func getState() *lua.LState {
	if state == nil {
		state = lua.NewState()
		setupPrint(state)
		setupHTTP(state)
		if code := getScriptFromDB(); code != "" {
			if err := state.DoString(code); err != nil {
				slog.Warn("lua: failed to load script from db", "err", err)
			}
		}
	}
	return state
}

func Handle(params responder.Responder) error {
	code := params.Match(1)

	if code == "reset" {
		Reset()
		params.Privmsgf(params.Target(), "lua state reset")
		return nil
	}

	result := Eval(code)
	params.Privmsgf(params.Target(), "%s", truncateForIRC(result))
	return nil
}

func truncateForIRC(out string) string {
	firstLine, rest, _ := strings.Cut(out, "\n")
	var suffix string
	if rest != "" {
		n := strings.Count(rest, "\n") + 1
		suffix = fmt.Sprintf(" [%d more lines...]", n)
	}
	if len(firstLine) > 420 {
		truncated := len(firstLine) - 420
		firstLine = firstLine[:420]
		charsSuffix := fmt.Sprintf(" [%d more chars]", truncated)
		suffix = charsSuffix + suffix
	}
	return firstLine + suffix
}

func Eval(code string) string {
	mu.Lock()
	defer mu.Unlock()

	outBuf.Reset()
	L := getState()

	returnFn, err := L.LoadString("return " + code)
	if err == nil {
		L.Push(returnFn)
		if err := L.PCall(0, lua.MultRet, nil); err != nil {
			return fmt.Sprintf("lua error: %s", err)
		}
		if L.GetTop() > 0 {
			n := L.GetTop()
			for i := 1; i <= n; i++ {
				if i > 1 {
					fmt.Fprint(&outBuf, "\t")
				}
				fmt.Fprint(&outBuf, L.ToStringMeta(L.Get(i)).String())
			}
			fmt.Fprintln(&outBuf)
			L.Pop(n)
		}
	} else {
		if err2 := L.DoString(code); err2 != nil {
			return fmt.Sprintf("lua error: %s", err2)
		}
	}

	out := strings.TrimSpace(outBuf.String())
	if out == "" {
		return "nil"
	}
	return out
}

func setupPrint(L *lua.LState) {
	printFn := L.NewFunction(func(L *lua.LState) int {
		top := L.GetTop()
		var parts []string
		for i := 1; i <= top; i++ {
			parts = append(parts, L.ToStringMeta(L.Get(i)).String())
		}
		fmt.Fprintln(&outBuf, strings.Join(parts, "\t"))
		return 0
	})
	L.SetGlobal("print", printFn)
}

func setupHTTP(L *lua.LState) {
	mod := L.NewTable()

	mod.RawSetString("get", L.NewFunction(luaHttpGet))
	mod.RawSetString("json", L.NewFunction(luaHttpJSON))

	L.SetGlobal("http", mod)
}

func luaHttpGet(L *lua.LState) int {
	url := L.CheckString(1)

	resp, err := httpClient.Get(url)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	L.Push(lua.LString(string(body)))
	return 1
}

func luaHttpJSON(L *lua.LState) int {
	url := L.CheckString(1)

	resp, err := httpClient.Get(url)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	lv := goToLua(L, data)
	L.Push(lv)
	return 1
}

func goToLua(L *lua.LState, v interface{}) lua.LValue {
	switch val := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(val)
	case float64:
		return lua.LNumber(val)
	case string:
		return lua.LString(val)
	case []interface{}:
		tbl := L.NewTable()
		for i, item := range val {
			tbl.RawSetInt(i+1, goToLua(L, item))
		}
		return tbl
	case map[string]interface{}:
		tbl := L.NewTable()
		for k, item := range val {
			tbl.RawSetString(k, goToLua(L, item))
		}
		return tbl
	default:
		return lua.LNil
	}
}

func getScriptFromDB() string {
	value, err := configs.Get("lua_script")
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Warn("lua: GetConfig", "err", err)
		}
		return ""
	}
	return value
}

func saveScriptToDB(code string) error {
	_, err := configs.Set("lua_script", code, "annnie")
	return err
}

// Reset destroys the Lua state and recreates it, reloading the persisted script.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	if state != nil {
		state.Close()
		state = nil
	}
	_ = getState()
}

func HandleReset(params responder.Responder) error {
	Reset()
	params.Privmsgf(params.Target(), "%s: lua state reset", params.Nick())
	return nil
}

// SaveScript persists the given Lua code to the config store and reloads it into the runtime.
// Returns an error if the code fails to parse or the DB write fails.
func SaveScript(code string) error {
	mu.Lock()
	// Validate by loading into a fresh state first
	testL := lua.NewState()
	setupPrint(testL)
	setupHTTP(testL)
	err := testL.DoString(code)
	testL.Close()
	if err != nil {
		mu.Unlock()
		return fmt.Errorf("lua parse error: %w", err)
	}

	// Persist
	if err := saveScriptToDB(code); err != nil {
		mu.Unlock()
		return fmt.Errorf("save to db: %w", err)
	}

	// Reload into runtime: reset and load fresh
	if state != nil {
		state.Close()
	}
	state = lua.NewState()
	setupPrint(state)
	setupHTTP(state)
	if err := state.DoString(code); err != nil {
		mu.Unlock()
		return fmt.Errorf("reload error: %w", err)
	}
	mu.Unlock()
	return nil
}

// GetScript returns the currently persisted Lua script.
func GetScript() string {
	return getScriptFromDB()
}
