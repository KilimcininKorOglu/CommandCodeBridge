package config

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// GetDateStr returns the current date in ISO format
func GetDateStr() string {
	return time.Now().Format("2006-01-02")
}

// GetEnvironment returns environment description
func GetEnvironment() string {
	return fmt.Sprintf("%s-%s, Go %s", runtime.GOOS, runtime.GOARCH, runtime.Version())
}

// GetWorkingDir returns the current working directory
func GetWorkingDir() (string, error) {
	return os.Getwd()
}

// GetGitInfo returns git repository information
func GetGitInfo() (isGitRepo bool, currentBranch, mainBranch, gitStatus string, recentCommits []string) {
	// Simplified implementation - in production, use git commands
	return false, "", "", "", []string{}
}
