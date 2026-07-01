package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSafePath(t *testing.T) {
	ws, _ := os.MkdirTemp("", "test-ws")
	defer os.RemoveAll(ws)

	p, err := resolveSafePath(ws, "main.go")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	expected := filepath.Join(ws, "main.go")
	if p != expected {
		t.Errorf("expected %s, got %s", expected, p)
	}
}

func TestResolveSafePathTraversal(t *testing.T) {
	ws, _ := os.MkdirTemp("", "test-ws")
	defer os.RemoveAll(ws)

	_, err := resolveSafePath(ws, "../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal, got nil")
	}
}

func TestResolveSafePathEmpty(t *testing.T) {
	_, err := resolveSafePath("/tmp", "")
	if err == nil {
		t.Error("expected error for empty path, got nil")
	}
}

func TestResolveSafePathNoWorkspace(t *testing.T) {
	_, err := resolveSafePath("", "main.go")
	if err == nil {
		t.Error("expected error for empty workspace, got nil")
	}
}

func TestReadFile(t *testing.T) {
	ws, _ := os.MkdirTemp("", "test-ws")
	defer os.RemoveAll(ws)
	os.WriteFile(filepath.Join(ws, "test.txt"), []byte("hello world"), 0644)

	content, err := readFile(ws, "test.txt")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if content != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", content)
	}
}

func TestReadFileNotFound(t *testing.T) {
	ws, _ := os.MkdirTemp("", "test-ws")
	defer os.RemoveAll(ws)

	_, err := readFile(ws, "nonexistent.go")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestReadFileIsDir(t *testing.T) {
	ws, _ := os.MkdirTemp("", "test-ws")
	defer os.RemoveAll(ws)
	os.Mkdir(filepath.Join(ws, "subdir"), 0755)

	_, err := readFile(ws, "subdir")
	if err == nil {
		t.Error("expected error for directory, got nil")
	}
}

func TestReadFileTraversal(t *testing.T) {
	ws, _ := os.MkdirTemp("", "test-ws")
	defer os.RemoveAll(ws)

	_, err := readFile(ws, "../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal, got nil")
	}
}

func TestReadFileForContext(t *testing.T) {
	ws, _ := os.MkdirTemp("", "test-ws")
	defer os.RemoveAll(ws)
	os.WriteFile(filepath.Join(ws, "main.go"), []byte("package main"), 0644)

	ctx := readFileForContext(ws, "main.go")
	if !strings.Contains(ctx, "package main") {
		t.Errorf("expected context to contain file content, got: %s", ctx)
	}
	if !strings.Contains(ctx, "main.go") {
		t.Errorf("expected context to contain file path, got: %s", ctx)
	}
	if !strings.Contains(ctx, "```go") {
		t.Errorf("expected context to have go code fence, got: %s", ctx)
	}
}

func TestReadFileForContextError(t *testing.T) {
	ws, _ := os.MkdirTemp("", "test-ws")
	defer os.RemoveAll(ws)

	ctx := readFileForContext(ws, "nonexistent.go")
	if !strings.Contains(ctx, "[error") {
		t.Errorf("expected error in context, got: %s", ctx)
	}
}
