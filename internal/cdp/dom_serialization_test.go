package cdp

import (
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestDOMOuterHTMLSerializesRequestedLiveNode(t *testing.T) {
	s, addr := runningServer(t)
	_, err := evaluatePageFixture(s.Page, `document.body.innerHTML='<article id="live">before<!--marker--></article>';document.querySelector('#live').firstChild.data='after'`)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := s.Page.Document()
	n, _ := d.Find("#live")
	c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/devtools/page/"+s.Page.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for index, id := range []int64{n.ID, d.Root().ID} {
		_ = c.WriteJSON(map[string]any{"id": index + 1, "method": "DOM.getOuterHTML", "params": map[string]any{"nodeId": id}})
		reply := readReply(t, c, float64(index+1))
		if reply["error"] != nil {
			t.Fatal(reply)
		}
		markup := reply["result"].(map[string]any)["outerHTML"].(string)
		if !strings.Contains(markup, `<article id="live">after<!--marker--></article>`) {
			t.Fatal(markup)
		}
		if index == 0 && strings.Contains(markup, "<html>") {
			t.Fatal("nodeId ignored")
		}
	}
	root := cdpNode(d, d.Root(), -1)
	if root["nodeType"] != 9 || root["nodeName"] != "#document" {
		t.Fatal(root)
	}
}
