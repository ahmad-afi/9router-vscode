package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
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

var httpClient = &http.Client{
	Timeout: 120 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

func validateEndpoint(rawURL, apiKey string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && scheme != "http" {
		return fmt.Errorf("endpoint must be http or https, got: %s", scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("endpoint has no host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			if scheme != "https" && apiKey != "" {
				return fmt.Errorf("refusing to send API key to non-HTTPS internal endpoint: %s", host)
			}
		}
	} else {
		ips, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("cannot resolve endpoint host: %w", err)
		}
		for _, ip := range ips {
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
				if scheme != "https" && apiKey != "" {
					return fmt.Errorf("refusing to send API key to non-HTTPS internal endpoint: %s (resolves to %s)", host, ip)
				}
			}
		}
	}
	if scheme != "https" && apiKey != "" {
		return fmt.Errorf("refusing to send API key over non-HTTPS connection: %s", rawURL)
	}
	return nil
}

func chat(reqID int, p ChatParams) error {
	if err := validateEndpoint(p.Endpoint, p.APIKey); err != nil {
		return writeResp(reqID, "error", ErrorParams{Message: err.Error()})
	}

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
		return writeResp(reqID, "error", ErrorParams{Message: err.Error()})
	}

	req, err := http.NewRequest("POST", p.Endpoint, bytes.NewReader(body))
	if err != nil {
		return writeResp(reqID, "error", ErrorParams{Message: err.Error()})
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return writeResp(reqID, "error", ErrorParams{Message: err.Error()})
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return writeResp(reqID, "error", ErrorParams{
			Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(b)),
		})
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var fullContent strings.Builder
	inTerminalBlock := false
	var terminalCmd strings.Builder

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
		if len(chunk.Choices) == 0 {
			continue
		}
		text := chunk.Choices[0].Delta.Content
		if text == "" {
			continue
		}
		fullContent.WriteString(text)

		if inTerminalBlock {
			if strings.Contains(text, "```") {
				inTerminalBlock = false
				cmd := strings.TrimSpace(terminalCmd.String())
				terminalCmd.Reset()
				if cmd != "" {
					if err := requestToolExec(reqID, cmd, p.WorkspacePath); err != nil {
						writeResp(reqID, "chat.chunk", ChunkParams{Text: "\n\n**Tool error:** " + err.Error() + "\n"})
					}
				}
				continue
			}
			terminalCmd.WriteString(text)
			continue
		}

		if strings.Contains(fullContent.String(), "```terminal") {
			idx := strings.LastIndex(fullContent.String(), "```terminal")
			after := fullContent.String()[idx:]
			if rest := strings.TrimPrefix(after, "```terminal"); rest != after {
				if strings.Contains(rest, "```") {
					lines := strings.SplitN(rest, "```", 2)
					cmd := strings.TrimSpace(lines[0])
					remaining := strings.TrimPrefix(lines[1], "```")
					if cmd != "" {
						if err := requestToolExec(reqID, cmd, p.WorkspacePath); err != nil {
							writeResp(reqID, "chat.chunk", ChunkParams{Text: "\n\n**Tool error:** " + err.Error() + "\n"})
						}
					}
					text = remaining
				} else {
					inTerminalBlock = true
					terminalCmd.WriteString(rest)
					continue
				}
			}
		}

		writeResp(reqID, "chat.chunk", ChunkParams{Text: text})
	}

	if err := scanner.Err(); err != nil {
		return writeResp(reqID, "error", ErrorParams{Message: err.Error()})
	}
	return writeResp(reqID, "chat.done", DoneParams{})
}

func requestToolExec(reqID int, command, workspacePath string) error {
	toolID := reqID*1000 + 1
	waiter := registerToolWaiter(toolID)

	writeResp(reqID, "chat.chunk", ChunkParams{Text: "\n\n**Executing:** `" + command + "`\n\n"})

	err := writeResp(toolID, "tool.terminal", ToolTerminalParams{
		Command:       command,
		WorkspacePath: workspacePath,
	})
	if err != nil {
		return err
	}

	select {
	case result := <-waiter:
		output := ""
		if result.Stdout != "" {
			output += "```\n" + result.Stdout + "```\n"
		}
		if result.Stderr != "" {
			output += "**stderr:**\n```\n" + result.Stderr + "```\n"
		}
		output += fmt.Sprintf("**Exit code:** %d\n\n", result.ExitCode)
		return writeResp(reqID, "chat.chunk", ChunkParams{Text: output})
	case <-time.After(60 * time.Second):
		return fmt.Errorf("tool execution timed out (no response from VSCode)")
	}
}
