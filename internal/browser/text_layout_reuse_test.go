package browser

import (
	"context"
	"testing"
)

func TestTextLayoutReuseTracksExactInputs(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), `(() => {
  const node = document.createElement('div');
  node.style.cssText = 'width:90px;font:16px Arial;line-height:20px';
  node.textContent = 'alpha beta gamma delta epsilon zeta eta theta';
  document.body.append(node);
  const measure = element => JSON.stringify([
    element.offsetWidth, element.offsetHeight, element.scrollWidth, element.scrollHeight
  ]);
  const changes = [
    () => {},
    () => { document.body.dataset.unrelated = 'changed'; },
    () => { node.style.width = '160px'; },
    () => { node.textContent += ' iota kappa lambda'; },
    () => { node.style.fontSize = '24px'; },
    () => { node.style.fontFamily = 'monospace'; },
    () => { node.style.fontWeight = '700'; },
    () => { node.style.fontStyle = 'italic'; },
    () => { node.style.whiteSpace = 'nowrap'; },
    () => { node.style.lineHeight = '40px'; },
    () => { node.style.fontSize = '0px'; },
    () => { node.style.fontSize = '16px'; node.style.whiteSpace = 'normal'; }
  ];
  for (let index = 0; index < changes.length; index++) {
    measure(node);
    changes[index]();
    const clone = node.cloneNode(true);
    document.body.append(clone);
    const actual = measure(node), fresh = measure(clone);
    if (actual !== fresh) return JSON.stringify({index, actual, fresh});
    clone.remove();
  }
  return 'ok';
})()`)
		if err != nil || value != "ok" {
			t.Fatalf("text layout reuse: %v %v", value, err)
		}
	})
}
