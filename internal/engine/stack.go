package engine

// NativeStackFrame is diagnostic data copied from the engine, without reading
// application Error.stack or invoking application formatting hooks.
type NativeStackFrame struct {
	Function string `json:"function"`
	URL      string `json:"url"`
	Line     int64  `json:"line"`
	Column   int64  `json:"column"`
	Source   string `json:"source,omitempty"`
}

// NativeStackCapture is optional and only callable inside a host callback.
type NativeStackCapture interface {
	CaptureNativeStack(limit int, sources bool) []NativeStackFrame
}
