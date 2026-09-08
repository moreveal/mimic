"""Capture bounded element layout reads before page scripts."""

import argparse
import asyncio
import json

from pyppeteer import connect


INSTRUMENT = r"""() => {
  const marker = '__MIMIC_LAYOUT__';
  const seen = new Set();
  let emitting = false;
  const emit = (kind, element, value) => {
    if (emitting) return;
    emitting = true;
    let computed = null;
    try { const css=getComputedStyle(element); computed={display:css.display,minHeight:css.minHeight,height:css.height,paddingTop:css.paddingTop,paddingBottom:css.paddingBottom,boxSizing:css.boxSizing}; } catch (_) {}
    let children = [];
    try { children=Array.from(element.children||[], child=>child.tagName); } catch (_) {}
    const item = {kind, tag: element && element.tagName, id: element && element.id,
      className: element && typeof element.className === 'string' ? element.className : '',
      style: element && element.getAttribute ? element.getAttribute('style') : null,
      parentTag: element && element.parentElement && element.parentElement.tagName,
      children, computed, value};
    const encoded = JSON.stringify(item);
    if (!seen.has(encoded)) { seen.add(encoded); console.debug(marker + encoded); }
    emitting = false;
  };
  const owner = [Element.prototype, HTMLElement.prototype].find(value => Object.prototype.hasOwnProperty.call(value, 'getBoundingClientRect'));
  if (owner) {
    const original = owner.getBoundingClientRect;
    Object.defineProperty(owner, 'getBoundingClientRect', {configurable:true, enumerable:true, writable:true,
      value: new Proxy(original, {apply(target, self, args) { const rect=Reflect.apply(target,self,args); emit('rect',self,{x:rect.x,y:rect.y,width:rect.width,height:rect.height,top:rect.top,right:rect.right,bottom:rect.bottom,left:rect.left}); return rect; }})});
  }
  for (const property of ['offsetWidth','offsetHeight']) {
    const descriptor = Object.getOwnPropertyDescriptor(HTMLElement.prototype, property);
    if (!descriptor || !descriptor.get) continue;
    const getter = descriptor.get;
    descriptor.get = new Proxy(getter, {apply(target,self,args) { const value=Reflect.apply(target,self,args); emit(property,self,value); return value; }});
    Object.defineProperty(HTMLElement.prototype, property, descriptor);
  }
  const computedStyle = globalThis.getComputedStyle;
  globalThis.getComputedStyle = new Proxy(computedStyle, {apply(target,self,args) {
    const style=Reflect.apply(target,self,args);
    return new Proxy(style,{get(object,property,receiver){
      const value=Reflect.get(object,property,receiver);
      if(typeof property==='string'&&typeof value!=='function')console.debug(marker+JSON.stringify({kind:'computed-property',property,value}));
      return value;
    }});
  }});
}"""


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--url", required=True)
    parser.add_argument("--settle-ms", type=int, default=15_000)
    args = parser.parse_args()
    browser = await connect(browserURL=args.endpoint, defaultViewport=None)
    page = await browser.newPage()
    observations = []
    page.on("console", lambda message: observations.append(json.loads(message.text[len('__MIMIC_LAYOUT__'):])) if message.text.startswith('__MIMIC_LAYOUT__') else None)
    await page._client.send("Network.clearBrowserCookies")
    await page.evaluateOnNewDocument(INSTRUMENT)
    error = None
    try:
        await page.goto(args.url, {"waitUntil": "load", "timeout": 120_000})
    except Exception as exc:
        error = str(exc)
    await asyncio.sleep(args.settle_ms / 1000)
    styles = await page.evaluate("() => Array.from(document.querySelectorAll('style'), node => node.textContent)")
    print(json.dumps({"error": error, "observations": observations, "styles": styles}, indent=2))
    await page.close()
    await browser.disconnect()


if __name__ == "__main__":
    asyncio.run(main())
