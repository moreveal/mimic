package cdp

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/moreveal/mimic/internal/browser"
)

type downloadPolicy struct {
	behavior, path string
	eventsEnabled  bool
}

func (s *Server) setDownloadBehavior(params map[string]any) error {
	contextID := stringValue(params["browserContextId"])
	if contextID != "" {
		if _, ok := s.Browser.Context(contextID); !ok {
			return fmt.Errorf("Failed to find context with id %s", contextID)
		}
	}
	behavior := stringValue(params["behavior"])
	if behavior != "allow" && behavior != "allowAndName" && behavior != "deny" && behavior != "default" {
		return fmt.Errorf("Invalid download behavior %q", behavior)
	}
	path := stringValue(params["downloadPath"])
	if (behavior == "allow" || behavior == "allowAndName") && path == "" {
		return fmt.Errorf("downloadPath is required for %s", behavior)
	}
	if path != "" && !filepath.IsAbs(path) {
		return fmt.Errorf("downloadPath must be absolute")
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.downloadPolicies == nil {
		s.downloadPolicies = make(map[string]downloadPolicy)
	}
	s.downloadPolicies[contextID] = downloadPolicy{behavior: behavior, path: path, eventsEnabled: params["eventsEnabled"] == true}
	return nil
}

func (s *Server) completeDownload(page *browser.Page, guid string) {
	download, ok := page.TakeDownload(guid)
	if !ok {
		return
	}
	s.lifecycleMu.Lock()
	policy, ok := s.downloadPolicies[page.ContextID()]
	if !ok {
		policy = s.downloadPolicies[""]
	}
	s.lifecycleMu.Unlock()
	if !policy.eventsEnabled {
		return
	}
	willBegin := map[string]any{"frameId": download.FrameID, "guid": guid, "url": download.URL, "suggestedFilename": download.SuggestedFilename}
	for _, client := range s.clientSnapshot() {
		if client.root != nil && client.root.browserSession {
			client.root.event("Browser.downloadWillBegin", willBegin)
		}
	}
	progress := map[string]any{"guid": guid, "totalBytes": len(download.Body), "receivedBytes": 0, "state": "canceled"}
	if policy.behavior == "allow" || policy.behavior == "allowAndName" {
		name := guid
		if policy.behavior == "allow" {
			name = download.SuggestedFilename
		}
		if err := os.MkdirAll(policy.path, 0o755); err == nil {
			file := filepath.Join(policy.path, name)
			if err = os.WriteFile(file, download.Body, 0o644); err == nil {
				progress["state"] = "completed"
				progress["receivedBytes"] = len(download.Body)
				progress["filePath"] = file
			}
		}
	}
	for _, client := range s.clientSnapshot() {
		if client.root != nil && client.root.browserSession {
			client.root.event("Browser.downloadProgress", progress)
		}
	}
}
