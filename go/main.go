package main

import (
	"bufio"
	"encoding/json"
	"os"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

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
		if err := chat(req.ID, p); err != nil {
			writeResp(req.ID, "error", ErrorParams{Message: err.Error()})
		}
		default:
			writeResp(req.ID, "error", ErrorParams{Message: "unknown method: " + req.Method})
		}
	}
}
