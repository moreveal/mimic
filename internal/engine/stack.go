package engine

// ExceptionDetails uses engine metadata rather than script-visible Error.stack.
// The value itself stays in its originating realm for ErrorEvent.error identity.
type ExceptionDetails struct {
	Message  string `json:"message"`
	Filename string `json:"filename"`
	Line     int    `json:"lineno"`
	Column   int    `json:"colno"`
}

type ExceptionInspector interface {
	DescribeException(Value) (ExceptionDetails, bool)
}

// ExceptionLocation preserves V8's own source location after an uncaught
// evaluation. It does not inspect the script-visible Error.stack property.
type ExceptionLocation interface {
	SourceLocation() (ExceptionDetails, []NativeStackFrame)
}

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
