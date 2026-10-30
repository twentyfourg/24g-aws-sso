package gsso

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// copyFile copies src to dst, preserving dst's directory creation but not
// src's mode (callers set what they need).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if info, err := in.Stat(); err == nil {
		_ = out.Chmod(info.Mode().Perm())
	}
	return nil
}

// CreateRollbackPoint copies ~/.aws/config into a timestamped backup
// directory before gsso changes it. Same format as 24GAWSSOSetup.md.
func CreateRollbackPoint(paths Paths) (string, error) {
	stamp := time.Now().Format("20060102-150405")
	backupDir := filepath.Join(paths.BackupRoot, stamp)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}
	var manifest []string
	if _, err := os.Stat(paths.ConfigPath); err == nil {
		if err := copyFile(paths.ConfigPath, filepath.Join(backupDir, "config")); err != nil {
			return "", err
		}
		manifest = append(manifest, fmt.Sprintf("config\t%s", paths.ConfigPath))
	} else {
		manifest = append(manifest, fmt.Sprintf("ABSENT\t%s", paths.ConfigPath))
	}
	manifestPath := filepath.Join(backupDir, "manifest.tsv")
	if err := os.WriteFile(manifestPath, []byte(strings.Join(manifest, "\n")+"\n"), 0o644); err != nil {
		return "", err
	}
	return backupDir, nil
}

// CreateMCPRollbackPoint backs up only the MCP configuration files a
// `gsso mcp` command is about to rewrite.
func CreateMCPRollbackPoint(paths Paths, files []string) (string, error) {
	stamp := time.Now().Format("20060102-150405")
	backupDir := filepath.Join(paths.MCPBackupRoot, stamp)
	for suffix := 1; ; suffix++ {
		if _, err := os.Stat(backupDir); os.IsNotExist(err) {
			break
		}
		backupDir = filepath.Join(paths.MCPBackupRoot, fmt.Sprintf("%s-%d", stamp, suffix))
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}
	var manifest []string
	for i, path := range files {
		backupName := fmt.Sprintf("%d-%s", i+1, filepath.Base(path))
		if err := copyFile(path, filepath.Join(backupDir, backupName)); err != nil {
			return "", err
		}
		manifest = append(manifest, fmt.Sprintf("%s\t%s", backupName, path))
	}
	manifestPath := filepath.Join(backupDir, "manifest.tsv")
	if err := os.WriteFile(manifestPath, []byte(strings.Join(manifest, "\n")+"\n"), 0o644); err != nil {
		return "", err
	}
	return backupDir, nil
}
