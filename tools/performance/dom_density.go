package main

import (
	"encoding/json"
	"github.com/moreveal/mimic/internal/dom"
	"os"
	"runtime"
	"strings"
	"time"
	"unsafe"
)

func main() {
	source := "<!doctype html><title>DOM density</title><body>" + strings.Repeat("<div class='row'><span>text</span></div>", 10000)
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	docs := make([]*dom.Document, 0, 5)
	for i := 0; i < 5; i++ {
		d, err := dom.Parse(source)
		if err != nil {
			panic(err)
		}
		docs = append(docs, d)
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	result := map[string]any{"nodeSize": unsafe.Sizeof(dom.Node{}), "documents": len(docs), "sourceBytes": len(source), "heapLiveDelta": after.HeapAlloc - before.HeapAlloc, "allocationDelta": after.TotalAlloc - before.TotalAlloc, "elapsedMillis": time.Since(start).Milliseconds()}
	json.NewEncoder(os.Stdout).Encode(result)
	runtime.KeepAlive(docs)
}
