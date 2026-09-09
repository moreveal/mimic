package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestDOMObservationsMatchChrome(t *testing.T) {
	fixture, err := os.ReadFile("testdata/dom_observations_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/dom-observations-chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var capture struct {
		Result map[string]any `json:"result"`
	}
	if err = json.Unmarshal(data, &capture); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), "JSON.stringify("+string(fixture)+")")
		if err != nil {
			t.Fatal(err)
		}
		var actual map[string]any
		if err = json.Unmarshal([]byte(value.(string)), &actual); err != nil {
			t.Fatal(err)
		}
		for key, want := range capture.Result {
			if !reflect.DeepEqual(actual[key], want) {
				t.Errorf("%s: got %v want %v", key, actual[key], want)
			}
		}
	})
}

func TestParsedDocumentOwnershipAndPosition(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), `(()=>{const parser=new DOMParser(),a=parser.parseFromString('<p id="x">first</p>','text/html'),b=parser.parseFromString('<p id="x">second</p>','text/html'),node=a.getElementById('x');if(node===b.getElementById('x')||node.ownerDocument!==a||node.parentNode!==a.body)return 'ownership';node.remove();if(node.ownerDocument!==a||node.isConnected)return 'removed';b.body.appendChild(node);if(node.ownerDocument!==b||b.body.lastChild!==node||node.textContent!=='first')return 'adoption';const x=a.createElement('x'),y=a.createElement('y'),xy=x.compareDocumentPosition(y),yx=y.compareDocumentPosition(x);if(!(xy&1)||!(xy&32)||xy!==x.compareDocumentPosition(y)||Boolean(xy&4)===Boolean(yx&4))return 'disconnected';const fragment=a.createDocumentFragment();fragment.append(x);if(fragment.compareDocumentPosition(x)!==20||x.compareDocumentPosition(fragment)!==10)return 'fragment';const xml=parser.parseFromString('<r xmlns:p="urn:p"><p:c p:a="v"/></r>','application/xml'),child=xml.documentElement.firstChild;if(child.localName!=='c'||child.prefix!=='p'||child.namespaceURI!=='urn:p'||child.getAttributeNS('urn:p','a')!=='v'||child.ownerDocument!==xml)return 'xml namespace';const bad=parser.parseFromString('<r>','application/xml');if(!bad.querySelector('parsererror'))return 'parser error';return true})()`)
		if err != nil || value != true {
			t.Fatalf("%v %v", value, err)
		}
	})
}
