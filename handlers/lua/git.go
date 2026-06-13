package lua

import (
	"fmt"
	"goirc/handlers/gitx"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func auth() *http.BasicAuth {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil
	}
	return &http.BasicAuth{Username: "x-access-token", Password: token}
}

// CloneOrPull ensures the git repo is cloned or up to date on startup.
func CloneOrPull() error {
	if gitx.GitRepo == "" {
		return fmt.Errorf("LUA_GIT_REPO not set")
	}
	remote := os.Getenv("LUA_GIT_REMOTE")
	if remote == "" {
		return fmt.Errorf("LUA_GIT_REMOTE not set")
	}

	if _, err := os.Stat(filepath.Join(gitx.GitRepo, ".git")); os.IsNotExist(err) {
		_, err := git.PlainClone(gitx.GitRepo, false, &git.CloneOptions{
			URL:  remote,
			Auth: auth(),
		})
		if err != nil {
			return fmt.Errorf("clone failed: %w", err)
		}
		return nil
	}

	repo, err := git.PlainOpen(gitx.GitRepo)
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

// func commitAndPush(code string) error {
// 	if gitx.GitRepo == "" {
// 		return fmt.Errorf("LUA_GIT_REPO not set")
// 	}

// 	scriptPath := filepath.Join(gitx.GitRepo, "script.lua")
// 	if err := os.WriteFile(scriptPath, []byte(code), 0644); err != nil {
// 		return fmt.Errorf("write file: %w", err)
// 	}

// 	repo, err := git.PlainOpen(gitx.GitRepo)
// 	if err != nil {
// 		return fmt.Errorf("open repo: %w", err)
// 	}
// 	w, err := repo.Worktree()
// 	if err != nil {
// 		return fmt.Errorf("worktree: %w", err)
// 	}

// 	if _, err := w.Add("script.lua"); err != nil {
// 		return fmt.Errorf("add: %w", err)
// 	}

// 	if _, err := w.Commit("update lua script", &git.CommitOptions{
// 		Author: &object.Signature{
// 			Name:  "annnie",
// 			Email: "annnie@ryanyeske.com",
// 			When:  time.Now(),
// 		},
// 	}); err != nil {
// 		return fmt.Errorf("commit: %w", err)
// 	}

// 	return repo.Push(&git.PushOptions{
// 		RemoteName: "origin",
// 		RefSpecs:   []config.RefSpec{config.RefSpec("refs/heads/main:refs/heads/main")},
// 		Auth:       auth(),
// 	})
// }
