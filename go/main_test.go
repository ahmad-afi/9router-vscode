package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)

func TestMainUnknownMethod(t *testing.T) {
	input := `{"id":1,"method":"unknown","params":{}}`
	out := runBinary(t, input)

	var resp Response
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}
	if resp.ID != 1 {
		t.Errorf("expected ID=1, got %d", resp.ID)
	}
	if resp.Method != "error" {
		t.Errorf("expected Method=error, got %s", resp.Method)
	}
	params, _ := resp.Params.(map[string]interface{})
	if !strings.Contains(params["message"].(string), "unknown method") {
		t.Errorf("expected 'unknown method' in message, got: %v", params["message"])
	}
}

func TestMainInvalidJSON(t *testing.T) {
	input := "not json at all"
	out := runBinary(t, input)

	var resp Response
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}
	if resp.Method != "error" {
		t.Errorf("expected Method=error, got %s", resp.Method)
	}
	params, _ := resp.Params.(map[string]interface{})
	if !strings.Contains(params["message"].(string), "invalid JSON") {
		t.Errorf("expected 'invalid JSON' in message, got: %v", params["message"])
	}
}

func TestMainEmptyLines(t *testing.T) {
	input := "\n\n\n"
	out := runBinary(t, input)

	if strings.TrimSpace(out) != "" {
		t.Errorf("expected no output for empty lines, got: %s", out)
	}
}

func TestMainMultipleRequests(t *testing.T) {
	input := `{"id":1,"method":"unknown","params":{}}` + "\n" +
		`{"id":2,"method":"unknown","params":{}}` + "\n"
	out := runBinary(t, input)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 response lines, got %d: %s", len(lines), out)
	}

	var resp1, resp2 Response
	json.Unmarshal([]byte(lines[0]), &resp1)
	json.Unmarshal([]byte(lines[1]), &resp2)

	if resp1.ID != 1 || resp2.ID != 2 {
		t.Errorf("expected IDs 1 and 2, got %d and %d", resp1.ID, resp2.ID)
	}
}

func TestMainChatBadParams(t *testing.T) {
	input := `{"id":5,"method":"chat","params":42}`
	out := runBinary(t, input)

	var resp Response
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}
	if resp.ID != 5 {
		t.Errorf("expected ID=5, got %d", resp.ID)
	}
	if resp.Method != "error" {
		t.Errorf("expected Method=error, got %s", resp.Method)
	}
}

func TestMainChatInvalidEndpoint(t *testing.T) {
	input := `{"id":10,"method":"chat","params":{"prompt":"hi","endpoint":"http://127.0.0.1:0","model":"m"}}`
	out := runBinary(t, input)

	if !contains(out, "chat.chunk") {
		// Should produce some error output
		if !contains(out, "error") {
			t.Errorf("expected error or chat output, got: %s", out)
		}
	}
}

func TestMainChatSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	input := `{"id":1,"method":"chat","params":{"prompt":"hello","endpoint":"` + srv.URL + `","model":"m"}}`
	out := runBinary(t, input)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines (chunk + done), got %d: %s", len(lines), out)
	}

	var chunkResp Response
	json.Unmarshal([]byte(lines[0]), &chunkResp)
	if chunkResp.Method != "chat.chunk" {
		t.Errorf("expected first line to be chat.chunk, got %s", chunkResp.Method)
	}

	var doneResp Response
	json.Unmarshal([]byte(lines[len(lines)-1]), &doneResp)
	if doneResp.Method != "chat.done" {
		t.Errorf("expected last line to be chat.done, got %s", doneResp.Method)
	}
}

func TestMainNDJSONMultiLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	input := `{"id":1,"method":"chat","params":{"prompt":"a","endpoint":"` + srv.URL + `","model":"m"}}` + "\n" +
		`{"id":2,"method":"unknown","params":{}}` + "\n"
	out := runBinary(t, input)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d: %s", len(lines), out)
	}
}

func TestMainLargeInput(t *testing.T) {
	large := strings.Repeat("x", 10000)
	input := `{"id":1,"method":"unknown","params":{"prompt":"` + large + `"}}`
	out := runBinary(t, input)

	var resp Response
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}
	if resp.Method != "error" {
		t.Errorf("expected error for unknown method, got %s", resp.Method)
	}
}

func runBinary(t *testing.T, input string) string {
	t.Helper()
	cmd := exec.Command("go", "run", ".")
	cmd.Stdin = bytes.NewReader([]byte(input))
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	if err := cmd.Run(); err != nil {
		// go run might return non-zero exit but still produce output
		t.Logf("go run returned error: %v", err)
	}
	return stdout.String()
}

func TestMainSingleRequest(t *testing.T) {
	input := `{"id":99,"method":"unknown","params":{}}`
	out := runBinary(t, input)

	var resp Response
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}
	if resp.ID != 99 {
		t.Errorf("expected ID=99, got %d", resp.ID)
	}
}
