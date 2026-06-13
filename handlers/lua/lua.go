package lua

import (
	"bytes"
	"encoding/json"
	"fmt"
	"goirc/handlers/gitx"
	"goirc/internal/responder"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

func getState() (*lua.LState, error) {
	if state == nil {
		state = lua.NewState()
		setupPrint(state)
		setupHTTP(state)
		if code, err := getScriptFromDB(); err != nil {
			return nil, fmt.Errorf("lua: getScriptFromDB: %w", err)
		} else if code != "" {
			if err := state.DoString(code); err != nil {
				return nil, fmt.Errorf("lua: DoString: %w", err)
			}
		}
	}
	return state, nil
}

func Handle(params responder.Responder) error {
	code := params.Match(1)

	if code == "reset" {
		Reset()
		params.Privmsgf(params.Target(), "lua state reset")
		return nil
	}

	result, err := Eval(code)
	if err != nil {
		return fmt.Errorf("Eval", err)
	}
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

func Eval(code string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	outBuf.Reset()
	L, err := getState()
	if err != nil {
		return "", fmt.Errorf("getState: %w", err)
	}

	returnFn, err := L.LoadString("return " + code)
	if err == nil {
		L.Push(returnFn)
		if err := L.PCall(0, lua.MultRet, nil); err != nil {
			return "", fmt.Errorf("lua error: %s", err)
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
			return "", fmt.Errorf("lua error2: %w", err2)
		}
	}

	out := strings.TrimSpace(outBuf.String())
	if out == "" {
		return "nil", nil
	}
	return out, nil
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

func getScriptFromDB() (string, error) {
	//q := model.New(db.DB.DB)
	// cfg, err := q.GetConfig(context.TODO(), "lua_script")
	// if err != nil {
	// 	return "", fmt.Errorf("get config lua_script: %w", err)
	// }
	// filename := cfg.Value
	// if filename == "" {
	// 	return "", fmt.Errorf("lua_script config is empty")
	// }
	filename := "script.lua"
	if gitx.GitRepo == "" {
		return "", fmt.Errorf("LUA_GIT_REPO not set")
	}
	body, err := os.ReadFile(filepath.Join(gitx.GitRepo, filename))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", filename, err)
	}
	return string(body), nil
}

// Reset destroys the Lua state and recreates it, reloading the persisted script.
func Reset() error {
	mu.Lock()
	defer mu.Unlock()
	if state != nil {
		state.Close()
		state = nil
	}
	_, err := getState()
	if err != nil {
		return fmt.Errorf("getState: %w", err)
	}
	return nil
}

// SaveScript persists the given Lua code to the config store and reloads it into the runtime.
// Returns an error if the code fails to parse or the DB write fails.
func SaveScript(code string) error {
	mu.Lock()
	defer mu.Unlock()

	// Validate by loading into a fresh state first
	testL := lua.NewState()
	setupPrint(testL)
	setupHTTP(testL)
	err := testL.DoString(code)
	testL.Close()
	if err != nil {
		return fmt.Errorf("lua parse error: %w", err)
	}

	// save script to disk
	scriptPath := filepath.Join(gitx.GitRepo, "script.lua")
	if err := os.WriteFile(scriptPath, []byte(code), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	// Reload into runtime: reset and load fresh
	if state != nil {
		state.Close()
	}
	state = lua.NewState()
	setupPrint(state)
	setupHTTP(state)
	if err := state.DoString(code); err != nil {
		return fmt.Errorf("reload error: %w", err)
	}

	return nil
}

// GetScript returns the currently persisted Lua script.
func GetScript() (string, error) {
	return getScriptFromDB()
}
