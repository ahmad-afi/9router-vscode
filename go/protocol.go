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
	Prompt        string `json:"prompt"`
	Selection     string `json:"selection"`
	Language      string `json:"language"`
	WorkspacePath string `json:"workspacePath"`
	Endpoint      string `json:"endpoint"`
	APIKey        string `json:"apiKey"`
	Model         string `json:"model"`
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

type ToolTerminalParams struct {
	Command       string `json:"command"`
	WorkspacePath string `json:"workspacePath"`
}

type ToolResultParams struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

var (
	pendingTools      = make(map[int]chan ToolResultParams)
	pendingToolsMutex sync.Mutex
)

func registerToolWaiter(id int) chan ToolResultParams {
	ch := make(chan ToolResultParams, 1)
	pendingToolsMutex.Lock()
	pendingTools[id] = ch
	pendingToolsMutex.Unlock()
	return ch
}

func deliverToolResult(id int, result ToolResultParams) bool {
	pendingToolsMutex.Lock()
	ch, ok := pendingTools[id]
	if ok {
		delete(pendingTools, id)
	}
	pendingToolsMutex.Unlock()
	if !ok {
		return false
	}
	ch <- result
	return true
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
