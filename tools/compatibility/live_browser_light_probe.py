"""Diagnostic only: rerun with text extraction limited to the selected element.

Separates whole-body clone/extraction cost from responsiveness. Do not mix its
timings with the main benchmark. Use github and spigotmc selectors only.
"""
import asyncio
import json
import live_browser_comparison as comparison


def light(selector):
    return """(() => {
      const el=document.querySelector(SELECTOR);
      const text=el?(el.textContent||'').replace(/\\s+/g,' ').trim():'';
      return {url:location.href,title:document.title,readyState:document.readyState,
        selectorCount:el?1:0,textLength:text.length,text:text.slice(0,14000),
        links:0,frames:0,challengeFrames:[],userAgent:navigator.userAgent};
    })()""".replace('SELECTOR',json.dumps(selector))


comparison.probe_expression=light
if __name__=='__main__':
    asyncio.run(comparison.main())
