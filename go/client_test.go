package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

	if !contains(out, "Hello") {
		t.Errorf("expected output to contain 'Hello', got: %s", out)
	}
	if !contains(out, " world") {
		t.Errorf("expected output to contain ' world', got: %s", out)
	}
	if !contains(out, "chat.done") {
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

	if !contains(out, "HTTP 500") {
		t.Errorf("expected output to contain 'HTTP 500', got: %s", out)
	}
	if !contains(out, "internal server error") {
		t.Errorf("expected output to contain error body, got: %s", out)
	}
	if !contains(out, "error") {
		t.Error("expected output to contain 'error' method")
	}
}

func TestChatEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{Endpoint: srv.URL, Prompt: "hi", Model: "m"})
	})

	if !contains(out, "chat.done") {
		t.Errorf("expected chat.done even on empty body, got: %s", out)
	}
}

func TestChatMalformedSSELines(t *testing.T) {
	sseBody := "garbage line\n\ndata: {not json}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{Endpoint: srv.URL, Prompt: "hi", Model: "m"})
	})

	if !contains(out, "ok") {
		t.Errorf("expected to extract 'ok' chunk despite malformed lines, got: %s", out)
	}
	if !contains(out, "chat.done") {
		t.Error("expected chat.done after processing")
	}
}

func TestChatDoneMarker(t *testing.T) {
	sseBody := "data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\ndata: [DONE]\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"b\"}}]}\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{Endpoint: srv.URL, Prompt: "hi", Model: "m"})
	})

	if contains(out, "\"b\"") {
		t.Errorf("should not process data after [DONE], got: %s", out)
	}
	if !contains(out, "\"a\"") {
		t.Error("expected to process chunk 'a' before [DONE]")
	}
}

func TestChatSSEEmptyDelta(t *testing.T) {
	sseBody := "data: {\"choices\":[{\"delta\":{\"content\":\"\"}}]}\n\ndata: [DONE]\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{Endpoint: srv.URL, Prompt: "hi", Model: "m"})
	})

	if !contains(out, "chat.done") {
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

	chat(1, ChatParams{Endpoint: srv.URL, APIKey: "mykey", Prompt: "hi", Model: "m"})
}

func TestChatContentTypeHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
		}
		w.Header().Set("Content-Type", "text/event-stream")
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

	if !contains(out, "error") {
		t.Errorf("expected error method for connection failure, got: %s", out)
	}
}

func TestChatRequestStructure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		t.Logf("request body: %s", string(body))
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	chat(1, ChatParams{Endpoint: srv.URL, Prompt: "test-prompt", Model: "test-model"})
}

func TestChatSSELargeChunk(t *testing.T) {
	large := string(make([]byte, 10000))
	for i := range large {
		large = large[:i] + "x" + large[i+1:]
	}
	sseBody := fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":\"%s\"}}]}\n\ndata: [DONE]\n\n", large)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	out := captureStdout(func() {
		chat(1, ChatParams{Endpoint: srv.URL, Prompt: "hi", Model: "m"})
	})

	if !contains(out, large) {
		t.Error("expected large chunk to be in output")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
