package browser

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestDevPreviewPublishesDuringContinuousCommands(t *testing.T) {
	b, err := NewWithOptions(v8engine.Factory{}, chrome152.New(), Options{DevPreview: true})
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	p.LockCommands()
	sub, err := p.SubscribePreview()
	p.UnlockCommands()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { p.LockCommands(); p.UnsubscribePreview(sub); p.UnlockCommands() }()
	// There is deliberately never a 50 ms quiet period, as on an active CDP Page.
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	updates := 0
	for {
		select {
		case <-ticker.C:
			p.LockCommands()
			p.UnlockCommands()
		case wire := <-sub.Updates:
			var packet map[string]any
			if err := json.Unmarshal(wire, &packet); err != nil {
				t.Fatal(err)
			}
			if packet["error"] != nil {
				t.Fatal(packet["error"])
			}
			updates++
			if updates == 2 {
				if !strings.Contains(packet["html"].(string), "ongoing update") {
					t.Fatal("missing changed state")
				}
				return
			}
			p.LockCommands()
			_, err := p.Evaluate(context.Background(), `document.body.textContent='ongoing update'`)
			p.UnlockCommands()
			if err != nil {
				t.Fatal(err)
			}
		case <-deadline.C:
			t.Fatalf("continuous commands starved preview after %d updates", updates)
		}
	}
}

func TestDevPreviewDisabledAndDirtyUpdates(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		b, err := NewWithOptions(v8engine.Factory{}, chrome152.New(), Options{DevPreview: enabled})
		if err != nil {
			t.Fatal(err)
		}
		c := b.NewContext()
		defer c.Close()
		p, err := c.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		p.LockCommands()
		sub, err := p.SubscribePreview()
		p.UnlockCommands()
		if !enabled {
			if err == nil || sub != nil || p.previewObservers != nil || p.Top.Realm.previewRead != nil {
				t.Fatal("disabled preview installed observation state")
			}
			if strings.Contains(p.Top.Realm.bootstrapSource().source, "devPreviewFormRevision") {
				t.Fatal("disabled bootstrap contains preview code")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		read := func() string {
			t.Helper()
			select {
			case wire := <-sub.Updates:
				var packet map[string]any
				if err := json.Unmarshal(wire, &packet); err != nil {
					t.Fatal(err)
				}
				if packet["error"] != nil {
					t.Fatal(packet["error"])
				}
				return packet["html"].(string)
			case <-time.After(2 * time.Second):
				t.Fatal("missing update")
				return ""
			}
		}
		read()
		p.LockCommands()
		p.UnlockCommands()
		select {
		case <-sub.Updates:
			t.Fatal("unchanged turn generated snapshot")
		default:
		}
		eval := func(js string) {
			t.Helper()
			p.LockCommands()
			_, e := p.Evaluate(context.Background(), js)
			p.UnlockCommands()
			if e != nil {
				t.Fatal(e)
			}
		}
		eval(`document.head.innerHTML='<style>body { background: red }</style>';document.body.innerHTML='<h1>Hello preview</h1><input id="field" value="old"><script>globalThis.shouldNotRun=1<\/script>';`)
		if html := read(); !strings.Contains(html, "Hello preview") || strings.Contains(html, "<script") {
			t.Fatalf("bad preview: %s", html)
		}
		eval(`document.getElementById('field').value='live';document.styleSheets[0].cssRules[0].style.backgroundColor='blue'`)
		if html := read(); !strings.Contains(html, `value="live"`) || !strings.Contains(html, "blue") {
			t.Fatalf("missing live form/CSSOM: %s", html)
		}
		eval(`document.styleSheets[0].disabled=true`)
		if html := read(); strings.Contains(html, "background: red") || strings.Contains(html, "blue") {
			t.Fatalf("disabled stylesheet survived: %s", html)
		}
		eval(`document.body.insertAdjacentHTML('beforeend','<div id="shadow"></div>');document.getElementById('shadow').attachShadow({mode:'open'}).innerHTML='<style>p{color:green}</style><p>Shadow text</p>'`)
		if html := read(); !strings.Contains(html, "shadowrootmode") || !strings.Contains(html, "Shadow text") {
			t.Fatalf("shadow projection missing: %s", html)
		}
		eval(`document.body.setAttribute('data-step','one')`)
		eval(`document.body.setAttribute('data-step','two')`)
		if html := read(); !strings.Contains(html, `data-step="two"`) {
			t.Fatal("mailbox did not keep latest state")
		}
		eval(`document.body.insertAdjacentHTML('beforeend','<dialog id="modal" data-mimic-preview-modal="author">Dialog</dialog>');document.getElementById('modal').showModal()`)
		if html := read(); !strings.Contains(html, `data-mimic-preview-modal="1"`) || strings.Contains(html, `data-mimic-preview-modal="author"`) {
			t.Fatal("canonical modal state not projected")
		}
		eval(`document.getElementById('modal').close()`)
		if html := read(); strings.Contains(html, `data-mimic-preview-modal=`) {
			t.Fatal("closed dialog still marked modal")
		}
		p.LockCommands()
		p.UnsubscribePreview(sub)
		p.UnlockCommands()
		if p.previewObservers != nil {
			t.Fatal("last unsubscribe retained preview observers")
		}
	}
}
