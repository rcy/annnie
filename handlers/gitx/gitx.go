package gitx

import (
	"fmt"
	"goirc/internal/responder"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

var GitRepo = os.Getenv("LUA_GIT_REPO")

var allowed = map[string]bool{
	"push":   true,
	"pull":   true,
	"status": true,
	"commit": true,
}

func auth() *http.BasicAuth {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil
	}
	return &http.BasicAuth{Username: "x-access-token", Password: token}
}

func Handle(params responder.Responder) error {
	sub := params.Match(1)

	args := strings.Fields(sub)
	if len(args) == 0 {
		params.Privmsgf(params.Target(), "usage: !git <push|pull|status|commit>")
		return nil
	}

	cmdName := args[0]
	if !allowed[cmdName] {
		params.Privmsgf(params.Target(), "disallowed git command: %s", cmdName)
		return nil
	}

	repo, err := gogit.PlainOpen(GitRepo)
	if err != nil {
		params.Privmsgf(params.Target(), "not a git repository: %s", err)
		return nil
	}

	switch cmdName {
	case "status":
		return handleStatus(params, repo)
	case "pull":
		return handlePull(params, repo)
	case "push":
		return handlePush(params, repo)
	case "commit":
		return handleCommit(params, repo, args[1:])
	}

	return nil
}

func handleStatus(params responder.Responder, repo *gogit.Repository) error {
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	status, err := w.Status()
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}

	if status.IsClean() {
		params.Privmsgf(params.Target(), "working tree clean")
		return nil
	}

	// Collect and sort paths for consistent output
	var paths []string
	for path := range status {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	//params.Privmsgf(params.Target(), "--- dirty files ---")
	for _, path := range paths {
		fs := status[path]
		params.Privmsgf(params.Target(), "  %c %s", fs.Worktree, path)
	}

	return nil
}

func handlePull(params responder.Responder, repo *gogit.Repository) error {
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	err = w.Pull(&gogit.PullOptions{Auth: auth()})
	if err == gogit.NoErrAlreadyUpToDate {
		params.Privmsgf(params.Target(), "already up to date")
		return nil
	}
	if err != nil {
		params.Privmsgf(params.Target(), "pull failed: %s", err)
		return nil
	}

	params.Privmsgf(params.Target(), "pull ok")
	return nil
}

func handleCommit(params responder.Responder, repo *gogit.Repository, msgParts []string) error {
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	msg := strings.Join(msgParts, " ")
	if msg == "" {
		msg = "update"
	}

	// Stage all changes
	status, err := w.Status()
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	if status.IsClean() {
		params.Privmsgf(params.Target(), "nothing to commit")
		return nil
	}

	for path := range status {
		if _, err := w.Add(path); err != nil {
			return fmt.Errorf("add %s: %w", path, err)
		}
	}

	_, err = w.Commit(msg, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "annnie",
			Email: "annnie@ryanyeske.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		params.Privmsgf(params.Target(), "commit failed: %s", err)
		return nil
	}

	params.Privmsgf(params.Target(), "committed: %s", msg)
	return nil
}

func handlePush(params responder.Responder, repo *gogit.Repository) error {
	err := repo.Push(&gogit.PushOptions{
		RemoteName: "origin",
		Auth:       auth(),
	})
	if err != nil {
		params.Privmsgf(params.Target(), "push failed: %s", err)
		return nil
	}

	params.Privmsgf(params.Target(), "push ok")
	return nil
}
