"""Measure layout wall time and process memory for DOM/static/Google snapshots."""
import argparse, asyncio, json, statistics, time
from pathlib import Path
import psutil
from pyppeteer import connect

DOM = '<main style="display:flex;width:760px">' + ''.join(
    f'<section style="display:flex;flex:1 1 0%;min-width:0"><span style="width:{20+i%30}px">item</span></section>' for i in range(600)) + '</main>'
STATIC = '<main style="display:grid;grid-template-columns:repeat(4,1fr);gap:8px;width:760px">' + ''.join(
    f'<article style="display:flex;flex-direction:column;min-width:0;padding:4px"><h3>Card {i}</h3><p>Static benchmark content</p></article>' for i in range(200)) + '</main>'
READ = """input => { if(input.invalidate)document.body.dataset.layoutIteration=String(input.iteration); const start=performance.now(); let sum=0; for(const node of document.querySelectorAll('body *')){const r=node.getBoundingClientRect();sum+=r.width+r.height+r.x+r.y} return {ms:performance.now()-start,sum}; }"""

async def measure(page, name, html=None, url=None):
    if html is not None: await page.setContent(html)
    else: await page.goto(url, {'waitUntil':'domcontentloaded','timeout':60000})
    cold=[];warm=[]
    for i in range(8):
        cold.append(await page.evaluate(READ, {'iteration':i,'invalidate':True}))
        warm.append(await page.evaluate(READ, {'iteration':i,'invalidate':False}))
    return {'workload':name,'layout_ms':{'first_cold':cold[0]['ms'],'median_cold':statistics.median(r['ms'] for r in cold[1:]),'median_warm':statistics.median(r['ms'] for r in warm)},'checksum':cold[-1]['sum']}

async def main(args):
    browser=await connect(browserURL=args.endpoint); process=psutil.Process(args.pid); before=process.memory_info(); rows=[]
    try:
        for name,html,url in [('dom',DOM,None),('static',STATIC,None),('google',None,'https://www.google.com/')]:
            rss_start=process.memory_info().rss;page=await browser.newPage(); start=time.perf_counter(); row=await measure(page,name,html,url);row['wall_ms']=(time.perf_counter()-start)*1000;row['rss_start']=rss_start;row['rss_live']=process.memory_info().rss;row['rss_delta']=row['rss_live']-rss_start;rows.append(row);await page.close();await asyncio.sleep(.25);row['rss_after_close']=process.memory_info().rss
    finally: await browser.disconnect()
    result={'endpoint':args.endpoint,'pid':args.pid,'rss_before':before.rss,'private_before':getattr(before,'private',None),'rows':rows,'rss_after':process.memory_info().rss}
    args.output.parent.mkdir(parents=True,exist_ok=True);args.output.write_text(json.dumps(result,indent=2),encoding='utf-8');print(json.dumps(result,indent=2))

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--endpoint',default='http://127.0.0.1:9222');parser.add_argument('--pid',type=int,required=True);parser.add_argument('--output',type=Path,required=True);asyncio.run(main(parser.parse_args()))
