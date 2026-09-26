package browser

import (
	"context"
	"testing"
)

func TestLegacyMouseAndUIEventInitialization(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	got, err := p.Evaluate(context.Background(), `(() => {
  for (const name of ['MouseEvent', 'MouseEvents', 'mouseevent']) {
   const e = document.createEvent(name);
   if (!(e instanceof MouseEvent) || e.screenY !== 0 || e.type !== '') return 'creation';
   e.initMouseEvent('x', true, true, window, 3, 10, 20, 30, 40, true, false, true, false, 2, document.body);
   if (e.view !== window || e.detail !== 3 || e.screenX !== 10 || e.screenY !== 20 || e.clientX !== 30 || e.clientY !== 40 || !e.ctrlKey || !e.shiftKey || e.altKey || e.metaKey || e.button !== 2 || e.which !== 3 || e.buttons !== 0 || e.relatedTarget !== document.body) return 'mouse initialization';
   e.preventDefault();
   document.body.addEventListener('x', () => e.initMouseEvent('ignored'), {once: true});
   document.body.dispatchEvent(e);
   if (e.type !== 'x' || !e.defaultPrevented) return 'dispatch guard';
   e.initMouseEvent('y');
   if (e.type !== 'y' || e.defaultPrevented || e.target !== null || e.screenY !== 0 || e.which !== 1 || e.view !== null) return 'reset';
  }
  for (const name of ['UIEvent', 'UIEvents']) {
   const e = document.createEvent(name);
   e.initUIEvent('ready', true, true, window, 4294967297);
   if (!(e instanceof UIEvent) || e.view !== window || e.detail !== 1 || !e.bubbles || !e.cancelable) return 'ui initialization';
  }
  if (UIEvent.prototype.initUIEvent.length !== 1 || MouseEvent.prototype.initMouseEvent.length !== 1) return 'metadata';
  return true;
 })()`)
	if err != nil || got != true {
		t.Fatalf("legacy events = %#v, %v", got, err)
	}
}
