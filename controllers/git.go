package controllers

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

func getLatestCommit(repo string) (string, error) {
	rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repo},
	})

	refs, err := rem.List(&git.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("listing remote refs for %s: %w", repo, err)
	}

	for _, ref := range refs {
		if ref.Name() == plumbing.HEAD {
			return ref.Hash().String(), nil
		}
	}

	return "", fmt.Errorf("HEAD not found in %s", repo)
}
