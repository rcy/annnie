package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

var gitRepo = os.Getenv("LUA_GIT_REPO")

func auth() *http.BasicAuth {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil
	}
	return &http.BasicAuth{Username: "x-access-token", Password: token}
}

// CloneOrPull ensures the git repo is cloned or up to date on startup.
func CloneOrPull() error {
	if gitRepo == "" {
		return fmt.Errorf("LUA_GIT_REPO not set")
	}
	remote := os.Getenv("LUA_GIT_REMOTE")
	if remote == "" {
		return fmt.Errorf("LUA_GIT_REMOTE not set")
	}

	if _, err := os.Stat(filepath.Join(gitRepo, ".git")); os.IsNotExist(err) {
		_, err := git.PlainClone(gitRepo, false, &git.CloneOptions{
			URL:  remote,
			Auth: auth(),
		})
		if err != nil {
			return fmt.Errorf("clone failed: %w", err)
		}
		return nil
	}

	repo, err := git.PlainOpen(gitRepo)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}
	err = w.Pull(&git.PullOptions{Auth: auth()})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("pull failed: %w", err)
	}
	return nil
}

func commitAndPush(code string) error {
	if gitRepo == "" {
		return fmt.Errorf("LUA_GIT_REPO not set")
	}

	scriptPath := filepath.Join(gitRepo, "script.lua")
	if err := os.WriteFile(scriptPath, []byte(code), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	repo, err := git.PlainOpen(gitRepo)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	if _, err := w.Add("script.lua"); err != nil {
		return fmt.Errorf("add: %w", err)
	}

	if _, err := w.Commit("update lua script", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "annnie",
			Email: "annnie@ryanyeske.com",
			When:  time.Now(),
		},
	}); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{config.RefSpec("refs/heads/main:refs/heads/main")},
		Auth:       auth(),
	})
}
