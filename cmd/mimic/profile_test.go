package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestProfileCLIProcess(t *testing.T) {
	path := os.Getenv("MIMIC_TEST_PROFILE_CLI")
	if path == "" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("mimic", flag.ExitOnError)
	os.Args = []string{"mimic", "--profile", path, "--listen", "127.0.0.1:0"}
	main()
	os.Exit(0)
}
func TestProfileCLIValidationAndDefault(t *testing.T) {
	for _, valid := range []bool{false, true} {
		t.Run(map[bool]string{false: "invalid", true: "valid"}[valid], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			source := `{"schemaVersion":1,"baseProfile":"chrome-152-windows-x64-headful-controlled-v1","window":{"viewportWidth":850}}`
			if !valid {
				source = `{"schemaVersion":1,"baseProfile":"unknown","network":{"proxy":{"password":"must-not-leak"}}}`
			}
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProfileCLIProcess$")
			cmd.Env = append(os.Environ(), "MIMIC_TEST_PROFILE_CLI="+path)
			if !valid {
				out, err := cmd.CombinedOutput()
				if err == nil || strings.Contains(string(out), "Mimic listening") || strings.Contains(string(out), "must-not-leak") || !strings.Contains(string(out), "baseProfile") {
					t.Fatal(string(out), err)
				}
				return
			}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { cancel(); cmd.Wait() }()
			scan := bufio.NewScanner(stdout)
			if !scan.Scan() {
				t.Fatal("no readiness")
			}
			address := strings.TrimPrefix(scan.Text(), "Mimic listening on ")
			response, err := http.Get(address + "/json/list")
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			var targets []struct {
				ID  string `json:"id"`
				URL string `json:"webSocketDebuggerUrl"`
			}
			if err = json.NewDecoder(response.Body).Decode(&targets); err != nil || len(targets) == 0 {
				t.Fatal(err)
			}
			ws, _, err := websocket.DefaultDialer.Dial(targets[0].URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ws.Close()
			ws.SetReadDeadline(time.Now().Add(5 * time.Second))
			ws.WriteJSON(map[string]any{"id": 1, "method": "Runtime.evaluate", "params": map[string]any{"expression": "innerWidth", "returnByValue": true}})
			for {
				var reply map[string]any
				if err = ws.ReadJSON(&reply); err != nil {
					t.Fatal(err)
				}
				if reply["id"] == float64(1) {
					result, ok := reply["result"].(map[string]any)
					if !ok || result["result"].(map[string]any)["value"] != float64(850) {
						t.Fatal(reply)
					}
					break
				}
			}
		})
	}
}
