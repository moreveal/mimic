package cdp

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestProtocolParameterValidationChrome152(t *testing.T) {
	raw, err := os.ReadFile("testdata/protocol_params_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name     string
			Method   string
			Params   *string
			Response struct {
				Error *protocolError
			}
		}
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, row := range fixture.Cases {
		t.Run(row.Name, func(t *testing.T) {
			var params json.RawMessage
			if row.Params != nil {
				params = json.RawMessage(*row.Params)
			}
			err := validateCommand(row.Method, params)
			observed := row.Response.Error
			if observed == nil || (observed.Code != -32601 && observed.Data == nil) {
				// Errors without deserialization data belong to the command's
				// semantics (e.g. missing objectId), not its generated decoder.
				if err != nil {
					t.Fatalf("Chrome accepted wire parameters, generated validator rejected: %v", err)
				}
				return
			}
			var actual *protocolError
			if !errors.As(err, &actual) || actual.Code != observed.Code {
				t.Fatalf("expected Chrome code %d; got %#v", observed.Code, err)
			}
			if actual.Code == -32602 && actual.Data == nil {
				t.Fatal("invalid params error must identify the failing field")
			}
		})
	}
}

func TestProtocolParameterNestedReferences(t *testing.T) {
	for _, row := range []struct {
		method string
		params string
		field  string
	}{
		{"Storage.setCookies", `{"cookies":[{"name":"a","value":"b","partitionKey":{"topLevelSite":false,"hasCrossSiteAncestor":true}}]}`, "params.cookies[0].partitionKey.topLevelSite"},
		{"Fetch.enable", `{"patterns":[{"requestStage":12}]}`, "params.patterns[0].requestStage"},
		{"Network.setCookies", `{"cookies":[{"name":"a"}]}`, "params.cookies[0].value"},
		{"Runtime.callFunctionOn", `{"functionDeclaration":"function(){}","arguments":[{"objectId":false}]}`, "params.arguments[0].objectId"},
		{"Runtime.evaluate", `{"expression":"1","timeout":1e999}`, "params.timeout"},
	} {
		err := validateCommand(row.method, json.RawMessage(row.params))
		var protocolErr *protocolError
		if !errors.As(err, &protocolErr) || protocolErr.Code != -32602 || !strings.Contains(protocolErr.Data.(string), row.field) {
			t.Errorf("%s: expected failure at %s, got %#v", row.method, row.field, err)
		}
	}
}

func TestProtocolSchemaIncludesCurrentBrowserAndAllV8Domains(t *testing.T) {
	// The historical stable-1.3 input omitted Fetch and the old PDL parser folded
	// Schema into Runtime and lost experimental members. These are real clients'
	// dependencies, not a threshold that can be met by extra placeholder names.
	for _, method := range []string{
		"Browser.getVersion", "Fetch.enable", "Target.createBrowserContext", "Target.createTarget",
		"Runtime.awaitPromise", "Runtime.getProperties", "Runtime.addBinding", "Runtime.getExceptionDetails",
		"Debugger.setInstrumentationBreakpoint", "Profiler.takePreciseCoverage", "Schema.getDomains",
	} {
		if _, ok := protocolCommands[method]; !ok {
			t.Errorf("missing pinned command %s", method)
		}
	}
	if _, ok := protocolCommands["Runtime.getDomains"]; ok {
		t.Fatal("malformed historical PDL command leaked into the current registry")
	}
	if _, ok := protocolCommands["Storage.enable"]; ok {
		t.Fatal("nonexistent Storage.enable must not be generated as a successful command")
	}
	if len(protocolCommands) != 665 || len(protocolEvents) != 234 || len(protocolTypes) != 611 {
		t.Fatalf("lost exact-Chrome schema entries: commands=%d events=%d types=%d", len(protocolCommands), len(protocolEvents), len(protocolTypes))
	}
	var schema struct {
		Domains []struct {
			Domain   string
			Commands []struct{ Name string }
			Events   []struct{ Name string }
		}
	}
	if err := json.Unmarshal(protocolDefinitionJSON, &schema); err != nil {
		t.Fatal(err)
	}
	for _, domain := range schema.Domains {
		for _, command := range domain.Commands {
			if _, ok := protocolCommands[domain.Domain+"."+command.Name]; !ok {
				t.Errorf("retained source and registry disagree: %s.%s", domain.Domain, command.Name)
			}
		}
		for _, event := range domain.Events {
			if _, ok := protocolEvents[domain.Domain+"."+event.Name]; !ok {
				t.Errorf("retained event source and registry disagree: %s.%s", domain.Domain, event.Name)
			}
		}
	}
	names := protocolCommandNames()
	delete(names, "Runtime.evaluate")
	if _, ok := protocolCommandNames()["Runtime.evaluate"]; !ok {
		t.Fatal("returned names expose mutable shared registry")
	}
}

func TestGeneratedProtocolTypesPreserveOmittedAndExplicitValues(t *testing.T) {
	var params DOMGetDocumentParams
	if err := json.Unmarshal([]byte(`{"depth":1e0,"pierce":false}`), &params); err != nil {
		t.Fatal(err)
	}
	if params.Depth == nil || *params.Depth != 1 || params.Pierce == nil || *params.Pierce {
		t.Fatalf("lost explicit values: %#v", params)
	}
	var omitted DOMGetDocumentParams
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil || omitted.Depth != nil || omitted.Pierce != nil {
		t.Fatalf("omitted fields became explicit: %#v; %v", omitted, err)
	}
	for _, raw := range []string{`1.5`, `2147483648`, `-2147483649`, `null`, `true`, `"1"`} {
		var id DOMNodeId
		if err := json.Unmarshal([]byte(raw), &id); err == nil {
			t.Errorf("generated integer ID accepted %s", raw)
		}
	}
	var argument RuntimeCallArgument
	if err := json.Unmarshal([]byte(`{"value":null}`), &argument); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(argument)
	if err != nil || !bytes.Equal(encoded, []byte(`{"value":null}`)) {
		t.Fatalf("explicit Runtime null was lost: %s; %v", encoded, err)
	}
	paramsForCookies := StorageSetCookiesParams{Cookies: []NetworkCookieParam{{Name: "a", Value: "b"}}}
	encoded, err = json.Marshal(paramsForCookies)
	if err != nil || validateCommand("Storage.setCookies", encoded) != nil {
		t.Fatalf("generated nested cookie types rejected: %s; %v", encoded, err)
	}
}

func TestProtocolBase64Validation(t *testing.T) {
	for _, value := range []string{"", "aGk=", "Zg==", "Zm9v", "Zh=="} {
		if !validProtocolBase64(value) {
			t.Errorf("valid base64 rejected: %q", value)
		}
	}
	for _, value := range []string{"aGk", "aGk=\n", "aGk=\r", "a Gk=", "====", "a==b", "a===", "_w==", "-w=="} {
		if validProtocolBase64(value) {
			t.Errorf("invalid base64 accepted: %q", value)
		}
	}
}
