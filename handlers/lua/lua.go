package lua

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"goirc/config"
	"goirc/internal/responder"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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

// ErrCommandNotFound is returned when a command name is not registered.
var ErrCommandNotFound = errors.New("command not found")

// Command describes a Lua command registered via register_command().
type Command struct {
	Name string
	Desc string
}

type luaCommand struct {
	fn   *lua.LFunction
	desc string
}

var registeredCommands = map[string]*luaCommand{}

func getState() (*lua.LState, error) {
	if state == nil {
		code, err := getScriptFromDB()
		if err != nil {
			return nil, fmt.Errorf("lua: getScriptFromDB: %w", err)
		}
		state = lua.NewState()
		if err := loadState(state, code); err != nil {
			return nil, fmt.Errorf("lua: DoString: %w", err)
		}
	}
	return state, nil
}

// loadState sets up the given state and loads the script, resetting the
// registered command registry so stale entries from a previous state are dropped.
func loadState(L *lua.LState, code string) error {
	setupPrint(L)
	setupHTTP(L)
	setupCommands(L)
	registeredCommands = map[string]*luaCommand{}
	if err := L.DoString(code); err != nil {
		return err
	}
	return nil
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
		return fmt.Errorf("Eval: %w", err)
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

// Dispatch is the IRC handler for registered Lua commands. It matches
// "!<name> [args...]" and silently ignores unknown command names.
func Dispatch(params responder.Responder) error {
	name := params.Match(1)
	args := params.Match(2)

	result, err := Invoke(name, args)
	if err != nil {
		if errors.Is(err, ErrCommandNotFound) {
			return nil
		}
		return err
	}
	params.Privmsgf(params.Target(), "%s", truncateForIRC(result))
	return nil
}

// Invoke calls a previously registered Lua command with a single string
// argument and returns the printed/returned output.
func Invoke(name, args string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	L, err := getState()
	if err != nil {
		return "", fmt.Errorf("getState: %w", err)
	}

	cmd, ok := registeredCommands[name]
	if !ok {
		return "", ErrCommandNotFound
	}

	return invoke(L, cmd, args)
}

// ListCommands returns the currently registered Lua commands sorted by name.
func ListCommands() []Command {
	mu.Lock()
	defer mu.Unlock()

	if _, err := getState(); err != nil {
		return nil
	}

	cmds := make([]Command, 0, len(registeredCommands))
	for name, cmd := range registeredCommands {
		cmds = append(cmds, Command{Name: name, Desc: cmd.desc})
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	return cmds
}

func invoke(L *lua.LState, cmd *luaCommand, args string) (string, error) {
	outBuf.Reset()

	L.Push(cmd.fn)
	L.Push(lua.LString(args))
	if err := L.PCall(1, lua.MultRet, nil); err != nil {
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

func setupCommands(L *lua.LState) {
	L.SetGlobal("register_command", L.NewFunction(luaRegisterCommand))
}

func luaRegisterCommand(L *lua.LState) int {
	name := L.CheckString(1)
	fn := L.CheckFunction(2)
	desc := ""
	if L.GetTop() >= 3 {
		desc = L.CheckString(3)
	}
	if !validCommandName(name) {
		L.RaiseError("register_command: invalid command name %q (use letters, digits, and underscores)", name)
		return 0
	}
	registeredCommands[name] = &luaCommand{fn: fn, desc: desc}
	return 0
}

func validCommandName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '_' {
			return false
		}
	}
	return true
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
	if config.Get().LuaGitRepo == "" {
		return "", fmt.Errorf("LUA_GIT_REPO not set")
	}
	body, err := os.ReadFile(filepath.Join(config.Get().LuaGitRepo, filename))
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
	err := loadState(testL, code)
	testL.Close()
	if err != nil {
		return fmt.Errorf("lua parse error: %w", err)
	}

	// save script to disk
	scriptPath := filepath.Join(config.Get().LuaGitRepo, "script.lua")
	if err := os.WriteFile(scriptPath, []byte(code), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	// Reload into runtime: reset and load fresh
	if state != nil {
		state.Close()
	}
	state = lua.NewState()
	if err := loadState(state, code); err != nil {
		return fmt.Errorf("reload error: %w", err)
	}

	return nil
}

// GetScript returns the currently persisted Lua script.
func GetScript() (string, error) {
	return getScriptFromDB()
}
