package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Clone clones a repository to the specified path.
// Deprecated: Use CloneMirror for bare repositories.
func Clone(url, path, branch string) error {
	cmd := exec.Command("git", "clone", "--branch", branch, url, path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w: %s", err, stderr.String())
	}

	return nil
}

// CloneMirror clones a repository as a bare mirror.
// Mirror clones fetch all refs and are ideal for read-only analysis.
func CloneMirror(url, path string) error {
	cmd := exec.Command("git", "clone", "--mirror", url, path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone --mirror failed: %w: %s", err, stderr.String())
	}

	return nil
}

// Pull pulls the latest changes for a repository.
// Deprecated: Use Fetch for bare repositories.
func Pull(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "pull")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull failed: %w: %s", err, stderr.String())
	}

	return nil
}

// Fetch fetches updates for a bare/mirror repository.
func Fetch(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "fetch", "--prune", "origin", "+refs/*:refs/*")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git fetch failed: %w: %s", err, stderr.String())
	}

	return nil
}

// SetRemoteURL updates the origin remote URL for a repository.
func SetRemoteURL(repoPath, newURL string) error {
	cmd := exec.Command("git", "-C", repoPath, "remote", "set-url", "origin", newURL)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git remote set-url failed: %w: %s", err, stderr.String())
	}

	return nil
}

// GetRemoteURL returns the current origin remote URL for a repository.
func GetRemoteURL(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git remote get-url failed: %w: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// GetFileContent retrieves the content of a file from HEAD in a bare repository.
func GetFileContent(repoPath, filepath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "show", "HEAD:"+filepath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git show HEAD:%s failed: %w: %s", filepath, err, stderr.String())
	}

	return stdout.String(), nil
}

// IsBareRepo checks if a repository is a bare repository.
func IsBareRepo(repoPath string) bool {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--is-bare-repository")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return false
	}

	return strings.TrimSpace(stdout.String()) == "true"
}

// CloneWithAuth clones a repository using an authenticated URL.
// The token is injected into the URL for authentication.
// Deprecated: Use CloneMirrorWithAuth for bare repositories.
func CloneWithAuth(url, path, branch, token string) error {
	authURL, err := injectToken(url, token)
	if err != nil {
		return fmt.Errorf("failed to create authenticated URL: %w", err)
	}

	cmd := exec.Command("git", "clone", "--branch", branch, authURL, path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w: %s", err, stderr.String())
	}

	// After cloning, set the remote URL to the original (non-authenticated) URL.
	// This prevents the token from being stored in .git/config.
	if err := SetRemoteURL(path, url); err != nil {
		return fmt.Errorf("failed to reset remote URL: %w", err)
	}

	return nil
}

// CloneMirrorWithAuth clones a repository as a bare mirror using an authenticated URL.
func CloneMirrorWithAuth(url, path, token string) error {
	authURL, err := injectToken(url, token)
	if err != nil {
		return fmt.Errorf("failed to create authenticated URL: %w", err)
	}

	cmd := exec.Command("git", "clone", "--mirror", authURL, path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone --mirror failed: %w: %s", err, stderr.String())
	}

	if err := SetRemoteURL(path, url); err != nil {
		return fmt.Errorf("failed to reset remote URL: %w", err)
	}

	return nil
}

// PullWithAuth pulls a repository using an authenticated URL.
// The token is temporarily injected for the pull operation.
// Deprecated: Use FetchWithAuth for bare repositories.
func PullWithAuth(repoPath, url, token string) error {
	authURL, err := injectToken(url, token)
	if err != nil {
		return fmt.Errorf("failed to create authenticated URL: %w", err)
	}

	if err := SetRemoteURL(repoPath, authURL); err != nil {
		return fmt.Errorf("failed to set authenticated URL: %w", err)
	}

	pullErr := Pull(repoPath)
	restoreErr := SetRemoteURL(repoPath, url)

	if pullErr != nil {
		return pullErr
	}
	if restoreErr != nil {
		return fmt.Errorf("failed to restore remote URL: %w", restoreErr)
	}

	return nil
}

// FetchWithAuth fetches a bare/mirror repository using an authenticated URL.
func FetchWithAuth(repoPath, url, token string) error {
	authURL, err := injectToken(url, token)
	if err != nil {
		return fmt.Errorf("failed to create authenticated URL: %w", err)
	}

	if err := SetRemoteURL(repoPath, authURL); err != nil {
		return fmt.Errorf("failed to set authenticated URL: %w", err)
	}

	fetchErr := Fetch(repoPath)
	restoreErr := SetRemoteURL(repoPath, url)

	if fetchErr != nil {
		return fetchErr
	}
	if restoreErr != nil {
		return fmt.Errorf("failed to restore remote URL: %w", restoreErr)
	}

	return nil
}

// FetchAll fetches all remote branches for a bare/mirror repository.
func FetchAll(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "fetch", "--prune", "origin", "+refs/*:refs/*")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git fetch failed: %w: %s", err, stderr.String())
	}

	return nil
}

// FetchAllWithAuth fetches all remote branches using an authenticated URL.
func FetchAllWithAuth(repoPath, url, token string) error {
	authURL, err := injectToken(url, token)
	if err != nil {
		return fmt.Errorf("failed to create authenticated URL: %w", err)
	}

	if err := SetRemoteURL(repoPath, authURL); err != nil {
		return fmt.Errorf("failed to set authenticated URL: %w", err)
	}

	fetchErr := FetchAll(repoPath)
	restoreErr := SetRemoteURL(repoPath, url)

	if fetchErr != nil {
		return fetchErr
	}
	if restoreErr != nil {
		return fmt.Errorf("failed to restore remote URL: %w", restoreErr)
	}

	return nil
}

// injectToken inserts an access token into a GitHub URL.
// Input: https://github.com/owner/repo.git
// Output: https://x-access-token:TOKEN@github.com/owner/repo.git
func injectToken(originalURL, token string) (string, error) {
	if !strings.HasPrefix(originalURL, "https://") {
		return "", fmt.Errorf("token injection only supported for HTTPS URLs")
	}
	return "https://x-access-token:" + token + "@" + strings.TrimPrefix(originalURL, "https://"), nil
}
