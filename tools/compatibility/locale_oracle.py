"""Compare context locale behavior with frozen Chrome 152 on an existing CDP endpoint.

Only owned temporary browser contexts are used and disposed. No existing target
is navigated. --output records a new diagnostic, never rewrites frozen fixtures.
"""
import argparse
import asyncio
import json
from pathlib import Path

import aiohttp

EXPRESSION = r"""(() => {
 const result={};
 const observe=(name,f)=>{try{result[name]=f()}catch(e){result[name]={error:e.name}}};
 const dates=['2026-01-01T12:34:56.789Z','2026-07-01T12:00:00Z','2026-03-08T02:30:00','2026-11-01T01:30:00','2026-01-01','1/1/2026 12:00','Jan 1 2026 12:00','2026-01-01T12:00:00+03:00','1900-01-01T00:00:00Z','2050-06-01T00:00:00Z','invalid'];
 for(const s of dates)observe(s,()=>{const d=new Date(s);return [d.getTime(),d.toString(),d.getTimezoneOffset(),d.toLocaleString(),d.toLocaleDateString(),d.toLocaleTimeString(),d.getFullYear(),d.getMonth(),d.getDay(),d.getDate(),d.getHours(),d.getMinutes(),d.getSeconds(),d.getMilliseconds(),Date.parse(s)]});
 for(const values of [[2026,2,8,2,30],[2026,10,1,1,30],[2011,11,30,12],[2026,3,5,1,45],[0,0],[99,11,32],[-1,0,1],[2026,0,1,0,0,0,-1]])observe('ctor'+values,()=>new Date(...values).toISOString());
 for(const ms of [-8640000000000000,8640000000000000])observe('edge'+ms,()=>{const d=new Date(ms);return [d.toString(),d.getFullYear(),d.getMonth(),d.getDate(),d.getTimezoneOffset()]});
 for(const name of ['setFullYear','setMonth','setDate','setHours','setMinutes','setSeconds','setMilliseconds','setYear'])for(const ms of [NaN,Date.parse('2026-03-08T06:30Z')])for(const args of [[],[2],[2026,2,8],[undefined],[1,undefined],[100,50,30,20]])observe(name+':'+ms+':'+args.map(String),()=>{const d=new Date(ms);return [d[name](...args),d.toString()]});
 observe('locale',()=>Intl.DateTimeFormat().resolvedOptions());
 observe('number',()=>[123456.78.toLocaleString(),new Intl.NumberFormat().format(123456.78),123456789n.toLocaleString()]);
 observe('inherited',()=>new Intl.DateTimeFormat(undefined,Object.create({year:'numeric',timeZone:'UTC'})).format(0));
 observe('explicit',()=>new Date(0).toLocaleString('de-DE',{timeZone:'UTC',dateStyle:'full',timeStyle:'long'}));
 observe('emptyLocales',()=>new Intl.NumberFormat([]).resolvedOptions().locale);
 observe('temporal',()=>typeof Temporal==='undefined'?null:[Temporal.Now.timeZoneId(),Temporal.Now.zonedDateTimeISO().timeZoneId,Temporal.Instant.from('2026-01-01T12:00Z').toLocaleString(),Temporal.PlainDate.from('2026-01-01').toLocaleString()]);
 observe('case',()=>['I'.toLocaleLowerCase(),'i'.toLocaleUpperCase(),new Intl.DateTimeFormat().constructor===Intl.DateTimeFormat]);
 observe('nullOptions',()=>new Intl.DateTimeFormat(undefined,null));
 return result;
})()"""

async def capture(port, mimic, expression=EXPRESSION, profiles=None):
    async with aiohttp.ClientSession() as http:
        async with http.get(f'http://127.0.0.1:{port}/json/version') as response:
            version = await response.json()
        if not mimic and not version['Browser'].startswith('Chrome/152.'):
            raise RuntimeError('Expected frozen Chrome 152: '+version['Browser'])
        async with http.ws_connect(version['webSocketDebuggerUrl']) as ws:
            seq = 0
            async def call(method, params=None, session=None):
                nonlocal seq
                seq += 1
                message = dict(id=seq, method=method, params=params or {})
                if session:
                    message['sessionId'] = session
                await ws.send_json(message)
                while True:
                    reply = json.loads((await ws.receive()).data)
                    if reply.get('id') == seq:
                        if 'error' in reply:
                            raise RuntimeError(reply['error'])
                        return reply['result']
            results = {}
            for zone, locale in profiles or [('America/New_York','en-US'),('Europe/Paris','fr-FR'),('Australia/Lord_Howe','en-AU'),('Pacific/Apia','en-US'),('Asia/Kathmandu','de-DE'),('UTC','en-US')]:
                if mimic:
                    schema = await call('Mimic.getProfileSchema')
                    context = (await call('Mimic.createContext',dict(profile=dict(schemaVersion=1,baseProfile=schema['baseProfiles'][0],locale=dict(timezone=zone,intlLocale=locale)))))['browserContextId']
                else:
                    context = (await call('Target.createBrowserContext'))['browserContextId']
                try:
                    target = (await call('Target.createTarget',dict(browserContextId=context,url='about:blank')))['targetId']
                    session = (await call('Target.attachToTarget',dict(targetId=target,flatten=True)))['sessionId']
                    if not mimic:
                        await call('Emulation.setTimezoneOverride',dict(timezoneId=zone),session)
                        await call('Emulation.setLocaleOverride',dict(locale=locale),session)
                    data=await call('Runtime.evaluate',dict(expression=expression,returnByValue=True,awaitPromise=True),session)
                    if 'exceptionDetails' in data:
                        raise RuntimeError(data['exceptionDetails'])
                    results[zone]=data['result']['value']
                finally:
                    await call('Target.disposeBrowserContext',dict(browserContextId=context))
            return {'browser':version['Browser'],'results':results}

async def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--chrome',type=int,default=9351)
    parser.add_argument('--mimic',type=int)
    parser.add_argument('--output',type=Path,required=True)
    args=parser.parse_args()
    result={'chrome':await capture(args.chrome,False)}
    if args.mimic:
        result['mimic']=await capture(args.mimic,True)
        diffs=[]
        for zone, observations in result['chrome']['results'].items():
            for key, expected in observations.items():
                actual=result['mimic']['results'][zone].get(key)
                if actual!=expected:
                    diffs.append(dict(zone=zone,key=key,expected=expected,actual=actual))
        result['differences']=diffs
        print(json.dumps(diffs[:12],ensure_ascii=True,indent=2))
        print('Differences:',len(diffs))
    args.output.write_text(json.dumps(result,ensure_ascii=True,indent=2)+'\n',encoding='utf-8')

if __name__=='__main__':
    asyncio.run(main())
