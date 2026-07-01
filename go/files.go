package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxFileSize = 64 * 1024

func resolveSafePath(workspacePath, relPath string) (string, error) {
	if workspacePath == "" {
		return "", fmt.Errorf("no workspace path")
	}
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return "", fmt.Errorf("empty file path")
	}
	abs, err := filepath.Abs(filepath.Join(workspacePath, relPath))
	if err != nil {
		return "", fmt.Errorf("cannot resolve path: %w", err)
	}
	wsAbs, err := filepath.Abs(workspacePath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve workspace: %w", err)
	}
	if !strings.HasPrefix(abs+string(filepath.Separator), wsAbs+string(filepath.Separator)) && abs != wsAbs {
		return "", fmt.Errorf("path traversal blocked: %s outside workspace", relPath)
	}
	return abs, nil
}

func readFile(workspacePath, relPath string) (string, error) {
	abs, err := resolveSafePath(workspacePath, relPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("not a file: %s", relPath)
	}
	if info.Size() > maxFileSize {
		return "", fmt.Errorf("file too large (%d bytes, max %d)", info.Size(), maxFileSize)
	}
	f, err := os.Open(abs)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFileSize))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func readFileForContext(workspacePath, relPath string) string {
	content, err := readFile(workspacePath, relPath)
	if err != nil {
		return fmt.Sprintf("[error reading %s: %v]", relPath, err)
	}
	ext := filepath.Ext(relPath)
	lang := strings.TrimPrefix(ext, ".")
	if lang == "" {
		lang = "text"
	}
	return fmt.Sprintf("\n\nFile `%s`:\n```%s\n%s\n```", relPath, lang, content)
}
