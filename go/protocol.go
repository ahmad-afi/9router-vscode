package main

import (
	"encoding/json"
	"os"
	"sync"
)

type Request struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type ChatParams struct {
	Prompt        string         `json:"prompt"`
	Selection     string         `json:"selection"`
	Language      string         `json:"language"`
	WorkspacePath string         `json:"workspacePath"`
	Endpoint      string         `json:"endpoint"`
	APIKey        string         `json:"apiKey"`
	Model         string         `json:"model"`
	Files         []string       `json:"files"`
	History       []HistoryEntry `json:"history"`
}

type Response struct {
	ID     int         `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type ChunkParams struct {
	Text string `json:"text"`
}

type ErrorParams struct {
	Message string `json:"message"`
}

type DoneParams struct{}

type HistoryEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ToolTerminalParams struct {
	Command       string `json:"command"`
	WorkspacePath string `json:"workspacePath"`
}

type ToolResultParams struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

type ApplyEditParams struct {
	FilePath string `json:"filePath"`
	Content  string `json:"content"`
}

type ApplyEditResultParams struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

var (
	pendingTools      = make(map[int]chan ToolResultParams)
	pendingToolsMutex sync.Mutex
)

func registerToolWaiter(id int) chan ToolResultParams {
	pendingToolsMutex.Lock()
	defer pendingToolsMutex.Unlock()
	if ch, ok := pendingTools[id]; ok {
		return ch
	}
	ch := make(chan ToolResultParams, 1)
	pendingTools[id] = ch
	return ch
}

func deliverToolResult(id int, result ToolResultParams) bool {
	pendingToolsMutex.Lock()
	ch, ok := pendingTools[id]
	pendingToolsMutex.Unlock()
	if !ok {
		return false
	}
	ch <- result
	return true
}

func cleanupToolWaiter(id int) {
	pendingToolsMutex.Lock()
	delete(pendingTools, id)
	pendingToolsMutex.Unlock()
}

func writeResp(id int, method string, params interface{}) error {
	resp := Response{ID: id, Method: method, Params: params}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(data, '\n'))
	return err
}
