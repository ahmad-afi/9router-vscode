package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestWriteRespFormat(t *testing.T) {
	out := captureStdout(func() {
		writeResp(1, "chat.chunk", ChunkParams{Text: "hello"})
	})

	var resp Response
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("writeResp output is not valid JSON: %v\noutput: %q", err, out)
	}
	if resp.ID != 1 {
		t.Errorf("expected ID=1, got %d", resp.ID)
	}
	if resp.Method != "chat.chunk" {
		t.Errorf("expected Method=chat.chunk, got %s", resp.Method)
	}
	params, ok := resp.Params.(map[string]interface{})
	if !ok {
		t.Fatalf("expected params to be a map, got %T", resp.Params)
	}
	if params["text"] != "hello" {
		t.Errorf("expected params.text=hello, got %v", params["text"])
	}
}

func TestWriteRespMultipleCalls(t *testing.T) {
	out := captureStdout(func() {
		writeResp(1, "chat.chunk", ChunkParams{Text: "a"})
		writeResp(2, "chat.chunk", ChunkParams{Text: "b"})
		writeResp(3, "chat.done", DoneParams{})
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	var resp1, resp2, resp3 Response
	json.Unmarshal([]byte(lines[0]), &resp1)
	json.Unmarshal([]byte(lines[1]), &resp2)
	json.Unmarshal([]byte(lines[2]), &resp3)

	if resp1.ID != 1 || resp1.Method != "chat.chunk" {
		t.Errorf("line 1 mismatch: %+v", resp1)
	}
	if resp2.ID != 2 || resp2.Method != "chat.chunk" {
		t.Errorf("line 2 mismatch: %+v", resp2)
	}
	if resp3.ID != 3 || resp3.Method != "chat.done" {
		t.Errorf("line 3 mismatch: %+v", resp3)
	}
}

func TestWriteRespTrailingNewline(t *testing.T) {
	out := captureStdout(func() {
		writeResp(0, "error", ErrorParams{Message: "test"})
	})
	if !strings.HasSuffix(out, "\n") {
		t.Error("writeResp output should end with newline")
	}
}

func TestWriteRespErrorParams(t *testing.T) {
	out := captureStdout(func() {
		writeResp(42, "error", ErrorParams{Message: "something broke"})
	})
	var resp Response
	json.Unmarshal([]byte(out), &resp)
	if resp.ID != 42 || resp.Method != "error" {
		t.Errorf("unexpected response: %+v", resp)
	}
	params, _ := resp.Params.(map[string]interface{})
	if params["message"] != "something broke" {
		t.Errorf("expected message 'something broke', got %v", params["message"])
	}
}

func TestRequestUnmarshalling(t *testing.T) {
	input := `{"id":42,"method":"chat","params":{"prompt":"hello","model":"gpt-4"}}`

	var req Request
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("failed to unmarshal Request: %v", err)
	}
	if req.ID != 42 {
		t.Errorf("expected ID=42, got %d", req.ID)
	}
	if req.Method != "chat" {
		t.Errorf("expected Method=chat, got %s", req.Method)
	}

	var p ChatParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		t.Fatalf("failed to unmarshal params: %v", err)
	}
	if p.Prompt != "hello" {
		t.Errorf("expected prompt=hello, got %s", p.Prompt)
	}
	if p.Model != "gpt-4" {
		t.Errorf("expected model=gpt-4, got %s", p.Model)
	}
}

func TestRequestWithNullParams(t *testing.T) {
	input := `{"id":1,"method":"chat","params":null}`
	var req Request
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if string(req.Params) != "null" && len(req.Params) != 0 {
		t.Errorf("expected null or empty params, got %s", string(req.Params))
	}
}

func TestChatParamsAllFields(t *testing.T) {
	input := `{"prompt":"p","selection":"s","language":"go","workspacePath":"/tmp","endpoint":"http://localhost","apiKey":"key","model":"m"}`

	var p ChatParams
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if p.Prompt != "p" {
		t.Errorf("prompt mismatch: %s", p.Prompt)
	}
	if p.Selection != "s" {
		t.Errorf("selection mismatch: %s", p.Selection)
	}
	if p.Language != "go" {
		t.Errorf("language mismatch: %s", p.Language)
	}
	if p.WorkspacePath != "/tmp" {
		t.Errorf("workspacePath mismatch: %s", p.WorkspacePath)
	}
	if p.Endpoint != "http://localhost" {
		t.Errorf("endpoint mismatch: %s", p.Endpoint)
	}
	if p.APIKey != "key" {
		t.Errorf("apiKey mismatch: %s", p.APIKey)
	}
	if p.Model != "m" {
		t.Errorf("model mismatch: %s", p.Model)
	}
}

func TestChatParamsEmpty(t *testing.T) {
	var p ChatParams
	if err := json.Unmarshal([]byte(`{}`), &p); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if p.Prompt != "" || p.Selection != "" || p.Language != "" ||
		p.WorkspacePath != "" || p.Endpoint != "" || p.APIKey != "" || p.Model != "" {
		t.Error("expected all fields to be empty")
	}
}
