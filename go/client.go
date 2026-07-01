package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

type nonStreamResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func chat(p ChatParams) error {
	system := "You are a helpful coding assistant."
	if p.Selection != "" {
		system += fmt.Sprintf("\n\nThe user has selected this code (%s):\n```%s\n%s\n```", p.Language, p.Language, p.Selection)
	}

	reqBody := chatRequest{
		Model: p.Model,
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: p.Prompt},
		},
		Stream: true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return writeResp(0, "error", ErrorParams{Message: err.Error()})
	}

	req, err := http.NewRequest("POST", p.Endpoint, bytes.NewReader(body))
	if err != nil {
		return writeResp(0, "error", ErrorParams{Message: err.Error()})
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return writeResp(0, "error", ErrorParams{Message: err.Error()})
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return writeResp(0, "error", ErrorParams{
			Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(b)),
		})
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			if err := writeResp(0, "chat.chunk", ChunkParams{Text: chunk.Choices[0].Delta.Content}); err != nil {
				return err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return writeResp(0, "error", ErrorParams{Message: err.Error()})
	}
	return writeResp(0, "chat.done", DoneParams{})
}
