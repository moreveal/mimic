package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"

	"github.com/gorilla/websocket"
)

type probe struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}
type target struct {
	WebSocketURL string `json:"webSocketDebuggerUrl"`
}
type versionInfo struct {
	Browser string `json:"Browser"`
}
type probeResult struct {
	Name        string `json:"name"`
	Passed      bool   `json:"passed"`
	Mimic       any    `json:"mimic,omitempty"`
	Chrome      any    `json:"chrome,omitempty"`
	MimicError  string `json:"mimicError,omitempty"`
	ChromeError string `json:"chromeError,omitempty"`
}
type reply struct {
	ID     int `json:"id"`
	Result struct {
		Result struct {
			Value       any    `json:"value"`
			Description string `json:"description"`
		} `json:"result"`
	} `json:"result"`
	Error any `json:"error"`
}

func discover(base string) (string, error) {
	res, err := http.Get(base + "/json/list")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var a []target
	if err := json.NewDecoder(res.Body).Decode(&a); err != nil {
		return "", err
	}
	if len(a) == 0 {
		return "", fmt.Errorf("no page targets at %s", base)
	}
	return a[0].WebSocketURL, nil
}
func browserVersion(base string) (string, error) {
	res, err := http.Get(base + "/json/version")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var info versionInfo
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return "", err
	}
	return info.Browser, nil
}
func evaluate(ws, expression string) (any, error) {
	c, _, err := websocket.DefaultDialer.Dial(ws, nil)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	if err := c.WriteJSON(map[string]any{"id": 1, "method": "Runtime.evaluate", "params": map[string]any{"expression": expression, "awaitPromise": true, "returnByValue": true}}); err != nil {
		return nil, err
	}
	for {
		_, body, err := c.ReadMessage()
		if err != nil {
			return nil, err
		}
		var r reply
		if json.Unmarshal(body, &r) == nil && r.ID == 1 {
			if r.Error != nil {
				return nil, fmt.Errorf("CDP: %v", r.Error)
			}
			return r.Result.Result.Value, nil
		}
	}
}
func main() {
	mimic := flag.String("mimic", "http://127.0.0.1:9222", "Mimic discovery URL")
	chrome := flag.String("chrome", "http://127.0.0.1:9223", "real Chrome discovery URL")
	file := flag.String("probes", "compatibility/probes.json", "probe file")
	targetVersion := flag.String("target-version", "152.0.7977.82", "required exact real Chrome version")
	output := flag.String("out", "", "optional JSON result artifact")
	flag.Parse()
	f, err := os.Open(*file)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	raw, _ := io.ReadAll(f)
	var probes []probe
	if err := json.Unmarshal(raw, &probes); err != nil {
		panic(err)
	}
	mw, err := discover(*mimic)
	if err != nil {
		panic(err)
	}
	cw, err := discover(*chrome)
	if err != nil {
		panic(err)
	}
	product, err := browserVersion(*chrome)
	if err != nil {
		panic(err)
	}
	if product != "Chrome/"+*targetVersion && product != "HeadlessChrome/"+*targetVersion {
		panic(fmt.Errorf("differential target mismatch: required Chrome/%s, endpoint reports %s", *targetVersion, product))
	}
	failed := 0
	results := make([]probeResult, 0, len(probes))
	for _, p := range probes {
		m, me := evaluate(mw, p.Expression)
		c, ce := evaluate(cw, p.Expression)
		ok := me == nil && ce == nil && reflect.DeepEqual(m, c)
		fmt.Printf("%-24s %v\n", p.Name, ok)
		if !ok {
			failed++
			fmt.Printf("  mimic=%#v err=%v\n  chrome=%#v err=%v\n", m, me, c, ce)
		}
		result := probeResult{Name: p.Name, Passed: ok, Mimic: m, Chrome: c}
		if me != nil {
			result.MimicError = me.Error()
		}
		if ce != nil {
			result.ChromeError = ce.Error()
		}
		results = append(results, result)
	}
	if *output != "" {
		artifact := map[string]any{"chromeVersion": *targetVersion, "chromeProduct": product, "probes": results}
		encoded, marshalErr := json.MarshalIndent(artifact, "", "  ")
		if marshalErr != nil {
			panic(marshalErr)
		}
		if writeErr := os.WriteFile(*output, append(encoded, '\n'), 0o644); writeErr != nil {
			panic(writeErr)
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
}
