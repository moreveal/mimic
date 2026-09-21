package browser

import (
	"mime"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/trace"
)

// Download retains the completed response until its CDP owner consumes it.
type Download struct {
	GUID, URL, SuggestedFilename, FrameID string
	Body                                  []byte
}

func downloadFilename(response network.Response) (string, bool) {
	disposition, params, err := mime.ParseMediaType(response.Headers.Get("Content-Disposition"))
	if err != nil || !strings.EqualFold(disposition, "attachment") {
		return "", false
	}
	name := path.Base(strings.ReplaceAll(params["filename"], "\\", "/"))
	if name == "." || name == "/" || name == "" {
		if response.URL != nil {
			name = path.Base(response.URL.Path)
		}
	}
	if name == "." || name == "/" || name == "" {
		name = "download"
	}
	return name, true
}

func (p *Page) captureDownload(response network.Response) bool {
	name, ok := downloadFilename(response)
	if !ok {
		return false
	}
	download := Download{GUID: uuid.NewString(), SuggestedFilename: name, FrameID: p.Top.ID, Body: response.Body}
	if response.URL != nil {
		download.URL = response.URL.String()
	}
	p.mu.Lock()
	if p.downloads == nil {
		p.downloads = make(map[string]Download)
	}
	p.downloads[download.GUID] = download
	p.mu.Unlock()
	p.trace.Add(trace.Lifecycle, "download", map[string]any{"frameId": download.FrameID, "guid": download.GUID})
	return true
}

func (p *Page) TakeDownload(guid string) (Download, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	download, ok := p.downloads[guid]
	delete(p.downloads, guid)
	return download, ok
}
