//go:build (windows || linux) && amd64

package v8

import "github.com/moreveal/mimic/internal/engine"

func (a *adapter) DescribeException(value engine.Value) (engine.ExceptionDetails, bool) {
	var out engine.ExceptionDetails
	callback := a.onCallback()
	if callback == nil {
		return out, false
	}
	local, err := a.localCallbackOrUndefined(callback.scope, value)
	if err != nil {
		return out, false
	}
	message, err := callback.ctx.CreateMessage(callback.scope.Scope(), local)
	if err != nil {
		return out, false
	}
	out.Message, err = message.Text(callback.ctx)
	if err != nil {
		return out, false
	}
	out.Filename, _ = message.ResourceName(callback.ctx)
	line, _, _ := message.LineNumber(callback.ctx)
	column, _ := message.StartColumn()
	out.Line = max(int(line), 0)
	out.Column = max(int(column)+1, 0)
	return out, true
}

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
