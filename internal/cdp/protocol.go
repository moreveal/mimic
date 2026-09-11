package cdp

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// These are immutable shared protocol descriptions, never Page state. The
// generated registry performs no startup JSON parsing and requires no locks.
//
//go:embed protocol/chrome152.json
var protocolDefinitionJSON []byte

//go:embed protocol_inventory_generated.json
var protocolInventoryJSON []byte

//go:generate python ../../tools/generate_cdp.py

type protocolKind uint8

const (
	protocolAny protocolKind = iota
	protocolString
	protocolInteger
	protocolNumber
	protocolBoolean
	protocolObject
	protocolArray
	protocolBinary
)

type protocolType struct {
	kind       protocolKind
	ref        string
	items      *protocolType
	properties []protocolField
}

type protocolField struct {
	name     string
	optional bool
	value    protocolType
}

type protocolCommand struct {
	parameters []protocolField
	returns    []protocolField
}

type protocolError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *protocolError) Error() string     { return e.Message }
func (e *protocolError) ProtocolCode() int { return e.Code }
func (e *protocolError) ProtocolData() any { return e.Data }

func protocolCommandNames() map[string]struct{} {
	out := make(map[string]struct{}, len(protocolCommands))
	for name := range protocolCommands {
		out[name] = struct{}{}
	}
	return out
}

func protocolEventNames() map[string]struct{} {
	out := make(map[string]struct{}, len(protocolEvents))
	for name := range protocolEvents {
		out[name] = struct{}{}
	}
	return out
}

func invalidProtocolParameter(path, detail string) error {
	return &protocolError{
		Code: -32602, Message: "Invalid parameters",
		Data: "Failed to deserialize " + path + " - " + detail,
	}
}

// validateCommand checks only the generated wire contract, not implementation
// support. Dispatch must still reject known commands without a real handler.
//
// Chrome's generated decoders ignore unknown fields, including nested fields.
// Enum strings are also decoded as strings: allowed values and relationships
// between optional arguments are command-specific semantic checks, not wire
// validation. See testdata/protocol_params_chrome152.json for exact observations.
func validateCommand(method string, raw json.RawMessage) error {
	command, exists := protocolCommands[method]
	if !exists {
		return &protocolError{Code: -32601, Message: "'" + method + "' wasn't found"}
	}
	// Commands without parameters don't deserialize params at all. Chrome accepts
	// even [] for Browser.getVersion, while DOM.getDocument with optional depth
	// accepts missing/null params but rejects [].
	if len(command.parameters) == 0 {
		return nil
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		raw = []byte("{}")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return invalidProtocolParameter("params", "object expected")
	}
	return validateProtocolFields(command.parameters, fields, "params", 0)
}

func validateProtocolFields(fields []protocolField, values map[string]json.RawMessage, path string, depth int) error {
	for i := range fields {
		field := &fields[i]
		value, exists := values[field.name]
		if !exists {
			if !field.optional {
				return invalidProtocolParameter(path+"."+field.name, "mandatory field missing")
			}
			continue
		}
		if err := validateProtocolValue(&field.value, value, path+"."+field.name, depth); err != nil {
			return err
		}
	}
	return nil
}

func validateProtocolValue(schema *protocolType, raw json.RawMessage, path string, depth int) error {
	// The JSON decoder already limits total JSON nesting. This bound additionally
	// prevents recursively described objects from consuming an unbounded stack.
	if depth > 256 {
		return invalidProtocolParameter(path, "maximum protocol nesting exceeded")
	}
	if schema.ref != "" {
		referenced, exists := protocolTypes[schema.ref]
		if !exists {
			return fmt.Errorf("generated CDP schema has unresolved reference %s", schema.ref)
		}
		return validateProtocolValue(&referenced, raw, path, depth+1)
	}
	raw = bytes.TrimSpace(raw)
	switch schema.kind {
	case protocolAny:
		// Parent unmarshalling already established valid JSON. Null is a real
		// Runtime value and must remain distinct from an absent optional value.
		return nil
	case protocolString:
		if len(raw) > 0 && raw[0] == '"' {
			return nil
		}
		return invalidProtocolParameter(path, "string value expected")
	case protocolBinary:
		var value string
		if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &value) != nil {
			return invalidProtocolParameter(path, "binary value expected")
		}
		if !validProtocolBase64(value) {
			return invalidProtocolParameter(path, "invalid base64 string")
		}
		return nil
	case protocolBoolean:
		if bytes.Equal(raw, []byte("true")) || bytes.Equal(raw, []byte("false")) {
			return nil
		}
		return invalidProtocolParameter(path, "bool value expected")
	case protocolInteger:
		if _, err := protocolInt32(raw); err == nil {
			return nil
		}
		return invalidProtocolParameter(path, "int32 value expected")
	case protocolNumber:
		value, err := strconv.ParseFloat(string(raw), 64)
		if err == nil && !math.IsInf(value, 0) && !math.IsNaN(value) {
			return nil
		}
		return invalidProtocolParameter(path, "double value expected")
	case protocolObject:
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
			return invalidProtocolParameter(path, "object expected")
		}
		return validateProtocolFields(schema.properties, fields, path, depth+1)
	case protocolArray:
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil || items == nil {
			return invalidProtocolParameter(path, "array expected")
		}
		for i, item := range items {
			if err := validateProtocolValue(schema.items, item, path+"["+strconv.Itoa(i)+"]", depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("generated CDP schema has unknown type %d", schema.kind)
	}
}

// CDPInteger retains Chrome's int32 wire semantics, including accepting JSON
// 1.0 and 1e0. A plain Go int32 rejects those valid protocol numbers on decode.
type CDPInteger int32

func protocolInt32(raw []byte) (int32, error) {
	value, err := strconv.ParseFloat(string(raw), 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) || value < math.MinInt32 || value > math.MaxInt32 || math.Trunc(value) != value {
		return 0, fmt.Errorf("int32 value expected")
	}
	return int32(value), nil
}

func (value *CDPInteger) UnmarshalJSON(raw []byte) error {
	parsed, err := protocolInt32(bytes.TrimSpace(raw))
	if err != nil {
		return err
	}
	*value = CDPInteger(parsed)
	return nil
}

func validProtocolBase64(value string) bool {
	// Validate without allocating a decoded response body. Chrome rejects both
	// unpadded base64 and whitespace (Go's base64 decoder ignores CR/LF).
	if len(value)%4 != 0 {
		return false
	}
	end := len(value)
	for padding := 0; padding < 2 && end > 0 && value[end-1] == '='; padding++ {
		end--
	}
	for i := 0; i < end; i++ {
		c := value[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/') {
			return false
		}
	}
	return true
}
