package controllers

import (
	"os/exec"
	"strings"
)

func getLatestCommit(repo string) (string, error) {

	out, err := exec.Command("git", "ls-remote", repo, "HEAD").Output()
	if err != nil {
		return "", err
	}

	parts := strings.Fields(string(out))
	if len(parts) == 0 {
		return "", nil
	}

	return parts[0], nil
}