#!/usr/bin/env python3
"""Capture trusted automation input in one fresh page of pinned headful Chrome."""
import argparse
import asyncio
import json
from pathlib import Path

import websockets

from capture_cdp_protocol import Connection, read_endpoint, write_json, ROOT


SETUP = """document.body.innerHTML='<input id="entry" value="ab"><input id="other"><textarea id="area"></textarea><input id="check" type="checkbox">';
globalThis.entry=document.querySelector('#entry');globalThis.other=document.querySelector('#other');
globalThis.inputEvents=[];
globalThis.inputEventTypes=['keydown','keypress','keyup','beforeinput','input','change','blur','focus','focusin','focusout','pointerdown','mousedown','pointerup','mouseup','click'];
if(globalThis.inputRecorder)for(const type of inputEventTypes)document.removeEventListener(type,inputRecorder,true);
globalThis.inputRecorder=e=>inputEvents.push({type:e.type,class:e.constructor.name,target:e.target.id,trusted:e.isTrusted,bubbles:e.bubbles,cancelable:e.cancelable,composed:e.composed,key:e.key??null,code:e.code??null,keyCode:e.keyCode??null,charCode:e.charCode??null,which:e.which??null,data:e.data??null,inputType:e.inputType??null,value:e.target.value??null,checked:e.target.type==='checkbox'?e.target.checked:null});
for(const type of inputEventTypes)document.addEventListener(type,inputRecorder,true);
entry.focus();inputEvents=[];"""

SCENARIOS = [
    ("insert-text-selection", "entry.setSelectionRange(1,2)", [("Input.insertText", {"text":"XY"})]),
    ("key-down-up", "", [
        ("Input.dispatchKeyEvent", {"type":"keyDown","key":"c","code":"KeyC","text":"c","unmodifiedText":"c","windowsVirtualKeyCode":67}),
        ("Input.dispatchKeyEvent", {"type":"keyUp","key":"c","code":"KeyC","windowsVirtualKeyCode":67}),
    ]),
    ("raw-down-char", "", [
        ("Input.dispatchKeyEvent", {"type":"rawKeyDown","key":"z","code":"KeyZ","windowsVirtualKeyCode":90}),
        ("Input.dispatchKeyEvent", {"type":"char","text":"z","key":"z","code":"KeyZ","windowsVirtualKeyCode":90}),
    ]),
    ("backspace", "entry.setSelectionRange(2,2)", [("Input.dispatchKeyEvent", {"type":"rawKeyDown","key":"Backspace","code":"Backspace","windowsVirtualKeyCode":8})]),
    ("prevent-keydown", "entry.addEventListener('keydown',e=>e.preventDefault())", [("Input.dispatchKeyEvent", {"type":"keyDown","key":"c","code":"KeyC","text":"c","windowsVirtualKeyCode":67})]),
    ("prevent-beforeinput", "entry.addEventListener('beforeinput',e=>e.preventDefault())", [("Input.insertText", {"text":"c"})]),
    ("edit-then-blur", "", [("Input.insertText", {"text":"c"}), ("Runtime.evaluate", {"expression":"other.focus()"})]),
    ("select-all-replace", "", [
        ("Input.dispatchKeyEvent", {"type":"rawKeyDown","key":"a","code":"KeyA","modifiers":2,"windowsVirtualKeyCode":65}),
        ("Input.insertText", {"text":"XY"}),
    ]),
    ("beforeinput-changes-selection", "entry.addEventListener('beforeinput',()=>entry.setSelectionRange(2,2))", [("Input.insertText", {"text":"X"})]),
    ("beforeinput-changes-value", "entry.addEventListener('beforeinput',()=>{entry.value='ZZ'})", [("Input.insertText", {"text":"X"})]),
    ("empty-insertion-removes-selection", "entry.setSelectionRange(1,2)", [("Input.insertText", {"text":""})]),
    ("maxlength-surrogate", "entry.value='';entry.setAttribute('maxlength','1')", [("Input.insertText", {"text":"😀"})]),
]

GEOMETRY = """document.body.innerHTML='<input id="text"><input id="small" size="10"><input id="checkbox" type="checkbox"><input id="radio" type="radio"><button id="go">Go</button><button id="long">Long label</button><input id="inputbutton" type="button" value="Go"><textarea id="textarea"></textarea><select id="select"><option>A</option><option>Longest</option></select>';JSON.stringify(Array.from(document.body.children,e=>{const s=getComputedStyle(e),r=e.getBoundingClientRect();return{id:e.id,width:r.width,height:r.height,fontSize:s.fontSize,fontFamily:s.fontFamily,lineHeight:s.lineHeight,padding:s.padding,border:s.borderWidth,boxSizing:s.boxSizing}}))"""


async def capture(endpoint):
    version=read_endpoint(endpoint,"/json/version")
    if version["Browser"]!="Chrome/152.0.7977.82": raise RuntimeError("wrong oracle version")
    async with websockets.connect(version["webSocketDebuggerUrl"],max_size=None) as socket:
        browser=Connection(socket)
        target=await browser.result("Target.createTarget",{"url":"about:blank"})
        try:
            info=next(row for row in read_endpoint(endpoint,"/json/list") if row['id']==target['targetId'])
            async with websockets.connect(info['webSocketDebuggerUrl'],max_size=None) as page_socket:
                page=Connection(page_socket)
                await page.result("Page.bringToFront")
                await page.result("Runtime.evaluate",{"expression":"new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))","awaitPromise":True})
                rows=[]
                for name,setup,commands in SCENARIOS:
                    await page.result("Runtime.evaluate",{"expression":SETUP+setup})
                    responses=[]
                    for method,params in commands:
                        responses.append(await page.send(method,params))
                    result=await page.result("Runtime.evaluate",{"expression":"({events:inputEvents,value:entry.value,start:entry.selectionStart,end:entry.selectionEnd,active:document.activeElement.id})","returnByValue":True})
                    rows.append({"name":name,"setup":setup,"commands":[{"method":m,"params":p} for m,p in commands],"result":result['result']['value']})
                provenance=json.loads((ROOT/'internal/cdp/protocol/chrome152.source.json').read_text(encoding='utf-8'))['captureMetadata']
                state=await page.result('Runtime.evaluate',{'expression':'({viewport:{width:innerWidth,height:innerHeight,deviceScaleFactor:devicePixelRatio},window:{x:screenX,y:screenY,outerWidth,outerHeight},secureContextState:isSecureContext,isolationState:crossOriginIsolated,visibilityState:document.visibilityState,hasFocus:document.hasFocus()})','returnByValue':True})
                provenance.update(state['result']['value'])
                geometry=await page.result('Runtime.evaluate',{'expression':GEOMETRY,'returnByValue':True})
                write_json(ROOT/'internal/browser/testdata/cdp_input_chrome152.json',{"captureMetadata":provenance,"setup":SETUP,"scenarios":rows,'geometrySetup':GEOMETRY,'geometry':json.loads(geometry['result']['value'])})
                print(json.dumps([{**{k:v for k,v in row.items() if k!='result'},'value':row['result']['value'],'events':[e['type'] for e in row['result']['events']]} for row in rows],ensure_ascii=True,indent=2))
        finally:
            await browser.result('Target.closeTarget',target)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--endpoint',required=True)
    asyncio.run(capture(parser.parse_args().endpoint))
