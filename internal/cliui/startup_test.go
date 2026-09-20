package cliui

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteStartupPlain(t *testing.T) {
	var output bytes.Buffer
	info := StartupInfo{Version: "v0.1.3", Chrome: 152, Engine: "v8", Address: "127.0.0.1:9222"}
	if err := WriteStartup(&output, info, false); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{"U|' \\/ '|u", "<<,-,,-..-,_|___|_,-.", "Mimic v0.1.3  ·  Chrome 152  ·  V8", "browser runtime · without Chromium", "◆ READY  CDP · 127.0.0.1:9222"} {
		if !strings.Contains(text, want) {
			t.Fatalf("startup banner is missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "\x1b[") {
		t.Fatalf("plain startup banner contains terminal styling: %q", text)
	}
}

func TestWriteStartupStyled(t *testing.T) {
	var output bytes.Buffer
	info := StartupInfo{Version: "dev", Chrome: 152, Engine: "goja", Address: "127.0.0.1:9222"}
	if err := WriteStartup(&output, info, true); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "\x1b[1;94m") || !strings.Contains(text, "\x1b[1;32m") || !strings.Contains(text, "\x1b[1;32m◆ READY") {
		t.Fatalf("styled startup banner is missing expected colors: %q", text)
	}
	if strings.Contains(text, "\x1b[1;32m"+logoGreen) {
		t.Fatalf("logo must use one blue color: %q", text)
	}
}

func TestBuildVersion(t *testing.T) {
	if version := BuildVersion(); version == "" {
		t.Fatal("empty build version")
	}
}
