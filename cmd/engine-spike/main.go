//go:build (windows || linux) && amd64

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	quickjsengine "github.com/moreveal/mimic/internal/engine/quickjs"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

type probe struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}

type observation struct {
	Value any    `json:"value"`
	Error string `json:"error"`
}

func main() {
	engineName := flag.String("engine", "v8", "engine to probe: v8 or quickjs")
	probesPath := flag.String("probes", "compatibility/probes-engine.json", "probe corpus")
	flag.Parse()
	data, err := os.ReadFile(*probesPath)
	check(err)
	var probes []probe
	check(json.Unmarshal(data, &probes))
	results := make(map[string]observation, len(probes))

	switch *engineName {
	case "v8":
		runtime, err := v8engine.NewRuntime()
		check(err)
		defer runtime.Dispose()
		realm, err := runtime.NewRealm()
		check(err)
		defer realm.Dispose()
		for _, probe := range probes {
			value, err := realm.Eval(probe.Expression, probe.Name+".js")
			results[probe.Name] = observed(value, err)
		}
	case "quickjs":
		runtime := (quickjsengine.Factory{}).New()
		defer runtime.Close()
		for _, probe := range probes {
			value, err := runtime.Eval(context.Background(), probe.Expression, probe.Name+".js")
			var exported any
			if value != nil {
				exported = value.Export()
			}
			results[probe.Name] = observed(exported, err)
		}
	default:
		check(fmt.Errorf("unknown engine %q", *engineName))
	}
	encoded, err := json.MarshalIndent(results, "", "  ")
	check(err)
	fmt.Println(string(encoded))
}

func observed(value any, err error) observation {
	if err != nil {
		return observation{Error: err.Error()}
	}
	return observation{Value: value}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
