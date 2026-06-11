package lua

import (
	"bytes"
	"encoding/json"
	"fmt"
	"goirc/internal/responder"
	"io"
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
	}
	return state
}

func Handle(params responder.Responder) error {
	code := params.Match(1)

	if code == "reset" {
		mu.Lock()
		if state != nil {
			state.Close()
			state = nil
		}
		mu.Unlock()
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
