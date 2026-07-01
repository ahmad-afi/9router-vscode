package main

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var wg sync.WaitGroup

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			writeResp(0, "error", ErrorParams{Message: "invalid JSON: " + err.Error()})
			continue
		}

		switch req.Method {
		case "chat":
			var p ChatParams
			if err := json.Unmarshal(req.Params, &p); err != nil {
				writeResp(req.ID, "error", ErrorParams{Message: err.Error()})
				continue
			}
			wg.Add(1)
			go func(id int, params ChatParams) {
				defer wg.Done()
				if err := chat(id, params); err != nil {
					writeResp(id, "error", ErrorParams{Message: err.Error()})
				}
			}(req.ID, p)
		case "tool.terminal":
			var p ToolTerminalParams
			if err := json.Unmarshal(req.Params, &p); err != nil {
				writeResp(req.ID, "error", ErrorParams{Message: err.Error()})
				continue
			}
			if err := runTerminal(req.ID, p.Command, p.WorkspacePath); err != nil {
				writeResp(req.ID, "error", ErrorParams{Message: err.Error()})
			}
		case "tool.result":
			var p ToolResultParams
			if err := json.Unmarshal(req.Params, &p); err != nil {
				writeResp(req.ID, "error", ErrorParams{Message: err.Error()})
				continue
			}
			if !deliverToolResult(req.ID, p) {
				writeResp(req.ID, "error", ErrorParams{Message: "no pending tool waiter for ID"})
			}
		case "apply.edit":
			var p ApplyEditParams
			if err := json.Unmarshal(req.Params, &p); err != nil {
				writeResp(req.ID, "error", ErrorParams{Message: err.Error()})
				continue
			}
			if err := requestApplyEdit(req.ID, p); err != nil {
				writeResp(req.ID, "error", ErrorParams{Message: err.Error()})
			}
		default:
			writeResp(req.ID, "error", ErrorParams{Message: "unknown method: " + req.Method})
		}
	}

	wg.Wait()
}
