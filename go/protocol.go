package main

import (
	"encoding/json"
	"os"
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

func writeResp(id int, method string, params interface{}) error {
	resp := Response{ID: id, Method: method, Params: params}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(data, '\n'))
	return err
}
