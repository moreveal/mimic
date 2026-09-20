package browser

import (
	"context"
	"testing"
)

func TestCustomizedBuiltInElementUpgrade(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	defer p.Close()
	value, err := p.Evaluate(context.Background(), `(()=>{let connected=0;class FancyButton extends HTMLButtonElement{connectedCallback(){connected++}}customElements.define('fancy-button',FancyButton,{extends:'button'});const button=document.createElement('button',{is:'fancy-button'});document.body.appendChild(button);return {instance:button instanceof FancyButton,name:button.localName,is:button.getAttribute('is'),connected}})()`)
	if err != nil {
		t.Fatal(err)
	}
	result := value.(map[string]any)
	if result["instance"] != true || result["name"] != "button" || result["is"] != "fancy-button" || result["connected"] != int64(1) {
		t.Fatalf("customized built-in did not upgrade: %#v", value)
	}
}
