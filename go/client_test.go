package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestChatSSEValid(t *testing.T) {
	sseBody := "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\ndata: [DONE]\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{
			Endpoint: srv.URL,
			Prompt:   "hi",
			Model:    "test-model",
		})
	})

	if !strings.Contains(out, "Hello") {
		t.Errorf("expected output to contain 'Hello', got: %s", out)
	}
	if !strings.Contains(out, " world") {
		t.Errorf("expected output to contain ' world', got: %s", out)
	}
	if !strings.Contains(out, "chat.done") {
		t.Error("expected output to contain chat.done")
	}
}

func TestChatNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "internal server error")
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{Endpoint: srv.URL, Prompt: "hi", Model: "m"})
	})

	if !strings.Contains(out, "HTTP 500") {
		t.Errorf("expected output to contain 'HTTP 500', got: %s", out)
	}
	if !strings.Contains(out, "internal server error") {
		t.Errorf("expected output to contain error body, got: %s", out)
	}
	if !strings.Contains(out, "error") {
		t.Error("expected output to contain 'error' method")
	}
}

func TestChatEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text-event-stream")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{Endpoint: srv.URL, Prompt: "hi", Model: "m"})
	})

	if !strings.Contains(out, "chat.done") {
		t.Errorf("expected chat.done even on empty body, got: %s", out)
	}
}

func TestChatMalformedSSELines(t *testing.T) {
	sseBody := "garbage line\n\ndata: {not json}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text-event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{Endpoint: srv.URL, Prompt: "hi", Model: "m"})
	})

	if !strings.Contains(out, "ok") {
		t.Errorf("expected to extract 'ok' chunk despite malformed lines, got: %s", out)
	}
	if !strings.Contains(out, "chat.done") {
		t.Error("expected chat.done after processing")
	}
}

func TestChatDoneMarker(t *testing.T) {
	sseBody := "data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\ndata: [DONE]\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"b\"}}]}\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text-event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{Endpoint: srv.URL, Prompt: "hi", Model: "m"})
	})

	if strings.Contains(out, "\"b\"") {
		t.Errorf("should not process data after [DONE], got: %s", out)
	}
	if !strings.Contains(out, "\"a\"") {
		t.Error("expected to process chunk 'a' before [DONE]")
	}
}

func TestChatSSEEmptyDelta(t *testing.T) {
	sseBody := "data: {\"choices\":[{\"delta\":{\"content\":\"\"}}]}\n\ndata: [DONE]\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text-event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{Endpoint: srv.URL, Prompt: "hi", Model: "m"})
	})

	if !strings.Contains(out, "chat.done") {
		t.Error("expected chat.done")
	}
}

func TestChatAPIKeyHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer mykey" {
			t.Errorf("expected Authorization header 'Bearer mykey', got '%s'", auth)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{Endpoint: srv.URL, APIKey: "mykey", Prompt: "hi", Model: "m"})
	})
	_ = out
}

func TestChatContentTypeHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
		}
		w.Header().Set("Content-Type", "text-event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	chat(1, ChatParams{Endpoint: srv.URL, Prompt: "hi", Model: "m"})
}

func TestChatConnectionError(t *testing.T) {
	out := captureStdout(func() {
		chat(1, ChatParams{Endpoint: "http://127.0.0.1:0", Prompt: "hi", Model: "m"})
	})

	if !strings.Contains(out, "error") {
		t.Errorf("expected error method for connection failure, got: %s", out)
	}
}

func TestChatRequestStructure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		t.Logf("request body: %s", string(body))
		w.Header().Set("Content-Type", "text-event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	chat(1, ChatParams{Endpoint: srv.URL, Prompt: "test-prompt", Model: "test-model"})
}

func TestChatSSELargeChunk(t *testing.T) {
	large := strings.Repeat("x", 10000)
	sseBody := fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":\"%s\"}}]}\n\ndata: [DONE]\n\n", large)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text-event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{Endpoint: srv.URL, Prompt: "hi", Model: "m"})
	})

	if !strings.Contains(out, large) {
		t.Error("expected large chunk to be in output")
	}
}

func TestChatWithHistory(t *testing.T) {
	sseBody := "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.Write([]byte(sseBody))
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{
			Endpoint: srv.URL,
			Prompt:   "follow up",
			Model:    "m",
			History: []HistoryEntry{
				{Role: "user", Content: "hello"},
				{Role: "assistant", Content: "hi there"},
			},
		})
	})

	if !strings.Contains(receivedBody, "hello") {
		t.Errorf("expected history 'hello' in request body, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, "hi there") {
		t.Errorf("expected history 'hi there' in request body, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, "follow up") {
		t.Errorf("expected prompt 'follow up' in request body, got: %s", receivedBody)
	}
	if !strings.Contains(out, "chat.done") {
		t.Error("expected chat.done")
	}
}

func TestChatWithFiles(t *testing.T) {
	dir := t.TempDir()
	testFile := "example.go"
	fullPath := dir + "/" + testFile
	if err := os.WriteFile(fullPath, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sseBody := "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 8192)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.Write([]byte(sseBody))
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{
			Endpoint:      srv.URL,
			Prompt:        "what is this",
			Model:         "m",
			WorkspacePath: dir,
			Files:         []string{testFile},
		})
	})

	if !strings.Contains(receivedBody, "package main") {
		t.Errorf("expected file content in request body, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, "example.go") {
		t.Errorf("expected file path in request body, got: %s", receivedBody)
	}
	if !strings.Contains(out, "chat.done") {
		t.Error("expected chat.done")
	}
}

func TestChatWithApplyEditBlock(t *testing.T) {
	toolCounter = 0
	toolID := 1*10000 + 1
	registerToolWaiter(toolID)
	go func() {
		deliverToolResult(toolID, ToolResultParams{Stdout: "ok", ExitCode: 0})
	}()

	sseBody := "data: {\"choices\":[{\"delta\":{\"content\":\"Here is the edit:\\n```apply\\nmain.go\\npackage main\\n```\\n\"}}]}\n\ndata: [DONE]\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.Write([]byte(sseBody))
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{
			Endpoint: srv.URL,
			Prompt:   "fix this",
			Model:    "m",
		})
	})

	if !strings.Contains(out, "apply.edit") {
		t.Errorf("expected apply.edit in output, got: %s", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("expected file path in output, got: %s", out)
	}
}

func TestRequestApplyEdit(t *testing.T) {
	toolCounter = 0
	toolID := 1*10000 + 1
	registerToolWaiter(toolID)
	go func() {
		deliverToolResult(toolID, ToolResultParams{Stdout: "ok", ExitCode: 0})
	}()

	out := captureStdout(func() {
		requestApplyEdit(1, ApplyEditParams{
			FilePath: "main.go",
			Content:  "package main",
		})
	})

	if !strings.Contains(out, "Applying edit") {
		t.Errorf("expected 'Applying edit' in output, got: %s", out)
	}
	if !strings.Contains(out, "apply.edit") {
		t.Errorf("expected apply.edit method in output, got: %s", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("expected file path in output, got: %s", out)
	}
}

func TestChatSSEMultipleTerminalBlocks(t *testing.T) {
	toolCounter = 0

	for i := 1; i <= 5; i++ {
		id := 1*10000 + i
		registerToolWaiter(id)
		go func(tid int) {
			deliverToolResult(tid, ToolResultParams{Stdout: "ok", ExitCode: 0})
		}(id)
	}

	sseBody := strings.Join([]string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"Run this:\\n```terminal\\necho hello\\n```\\nand this:\\n```terminal\\necho world\\n```\\n\"}}]}",
		"data: [DONE]",
	}, "\n\n") + "\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text-event-stream")
		w.WriteHeader(200)
		w.Write([]byte(sseBody))
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{
			Endpoint: srv.URL,
			Prompt:   "run commands",
			Model:    "m",
		})
	})

	count := strings.Count(out, "tool.terminal")
	if count < 2 {
		t.Errorf("expected at least 2 tool.terminal requests, got %d: %s", count, out)
	}
}
