// Package compatibility defines the version-neutral boundary between the
// runtime core and a concrete, generated Chrome compatibility bundle.
package compatibility

import (
	"net/http"

	"github.com/moreveal/mimic/internal/state"
)

type ChromeVersion struct {
	Milestone        int    `json:"milestone"`
	Version          string `json:"version"`
	ChromiumCommit   string `json:"chromiumCommit"`
	ChromiumRevision int    `json:"chromiumRevision"`
}

type WebAPISurface struct {
	GeneratedJavaScript string
	// Immutable generator input is transported separately from executable
	// source, so retained functions do not root its escaped source literal.
	GeneratedCatalogJSON string
	// Exposures records effective, version/platform/feature-gated globals from
	// the pinned browser. WebIDL remains the declaration source; these profiles
	// provide the runtime-enabled exposure decision that IDL alone cannot make.
	Exposures map[string]RealmExposure
}

type SurfaceProperty struct {
	Name           string `json:"name"`
	Enumerable     bool   `json:"enumerable"`
	Configurable   bool   `json:"configurable"`
	Writable       *bool  `json:"writable"`
	Getter         bool   `json:"getter"`
	Setter         bool   `json:"setter"`
	ValueType      string `json:"valueType"`
	FunctionName   string `json:"functionName"`
	FunctionLength *int   `json:"functionLength"`
}

type RealmExposure struct {
	Properties []SurfaceProperty            `json:"properties"`
	Prototypes map[string][]SurfaceProperty `json:"prototypes,omitempty"`
}

type ProtocolSchema struct {
	Methods map[string]struct{}
	Events  map[string]struct{}
}

type EnvironmentProfile struct {
	ID    string
	Mode  state.BrowserMode
	State state.Environment
	// NewTransport constructs the wire implementation selected by this exact
	// compatibility bundle. The browser core only consumes net/http's neutral
	// RoundTripper boundary and never imports a version package.
	NewTransport func() (http.RoundTripper, error)
}

type CompatExpectations struct {
	Platform      string
	Channel       string
	PrimaryOracle OracleDescriptor
	Sources       map[state.BrowserMode]OracleExpectationSource
}

type OracleDescriptor struct {
	ChromeVersion        string
	ChromiumRevision     int
	V8Version            string
	Platform             string
	Mode                 state.BrowserMode
	EnvironmentProfileID string
	Authoritative        bool
}

type OracleExpectationSource struct {
	Mode          state.BrowserMode
	CapturePath   string
	Authoritative bool
}

type Bundle interface {
	Version() ChromeVersion
	Surface() *WebAPISurface
	CDP() *ProtocolSchema
	Environment() *EnvironmentProfile
	Expectations() *CompatExpectations
}
