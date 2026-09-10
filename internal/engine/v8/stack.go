//go:build windows && amd64

package v8

import "github.com/moreveal/mimic/internal/engine"

func (a *adapter) CaptureNativeStack(limit int, sources bool) []engine.NativeStackFrame {
	callback := a.onCallback()
	if callback == nil {
		return nil
	}
	trace, ok, err := callback.scope.Scope().CurrentStackTrace(min(max(limit, 0), 32))
	if err != nil || !ok {
		return nil
	}
	count, err := trace.FrameCount()
	if err != nil {
		return nil
	}
	out := make([]engine.NativeStackFrame, 0, count)
	for i := 0; i < count; i++ {
		f, err := trace.Frame(i)
		if err != nil {
			break
		}
		var row engine.NativeStackFrame
		row.Function, _, _ = f.FunctionName()
		row.URL, _, _ = f.ScriptNameOrSourceURL()
		row.Line, _ = f.LineNumber()
		row.Column, _ = f.Column()
		if sources {
			row.Source, _, _ = f.ScriptSource()
			if len(row.Source) > 2<<20 {
				row.Source = ""
			}
		}
		out = append(out, row)
	}
	return out
}
