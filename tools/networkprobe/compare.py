"""Compare captured ClientHello observations; retain volatile fields separately."""
import base64
import hashlib
import json
import pathlib
import sys
from decode import decode, hello, varint

def grease(n): return n & 0x0f0f == 0x0a0a and n >> 8 == n & 255
def words(b): return [int.from_bytes(b[i:i+2],'big') for i in range(0,len(b),2)]
def ja4(h, protocol):
    ciphers=sorted(x for x in h['ciphers'] if not grease(int(x,16)))
    exts={e['id']:bytes.fromhex(e['data']) for e in h['extensions'] if not grease(e['id'])}
    ids=','.join(f'{n:04x}' for n in sorted(exts) if n not in (0,16))
    sigs=','.join(f'{n:04x}' for n in words(exts.get(13,b'\0\0')[2:]) if not grease(n))
    alpn=exts.get(16,b'\0\0\0')[3:]; n=exts.get(16,b'\0\0\0')[2]; alpn=alpn[:n]
    a=(chr(alpn[0])+chr(alpn[-1])) if alpn else '00'
    version=max((n for n in words(exts.get(43,b'')[1:]) if not grease(n)),default=0x303)
    prefix=f'{protocol}{ {0x304:"13",0x303:"12"}.get(version,"00")}{"d" if 0 in exts else "i"}{len(ciphers):02}{len(exts):02}{a}'
    return prefix+'_'+hashlib.sha256(','.join(ciphers).encode()).hexdigest()[:12]+'_'+hashlib.sha256((ids+'_'+sigs).encode()).hexdigest()[:12]

def normalize(h):
    result={'sessionIDLength':h['sessionIDLength'],'ciphers':[c for c in h['ciphers'] if not grease(int(c,16))],'extensions':{}}
    for e in h['extensions']:
        ident=e['id'];b=bytes.fromhex(e['data'])
        if grease(ident):continue
        v=e['data']
        if ident==51:
            p=2;v=[]
            while p<len(b):
                group=int.from_bytes(b[p:p+2],'big');n=int.from_bytes(b[p+2:p+4],'big');p+=4+n
                if not grease(group):v.append([group,n])
        elif ident==65037:v={'kdfAEAD':b[:5].hex()}
        elif ident==51764:
            p=2;v=[]
            while p<len(b):n=b[p];p+=1;v.append(b[p:p+n].hex());p+=n
            v.sort()
        elif ident in (10,13,43):v=[n for n in words(b[1 if ident==43 else 2:]) if not grease(n)]
        elif ident==57:
            p=0;v={}
            while p<len(b):
                tid,p=varint(b,p);n,p=varint(b,p);val=b[p:p+n];p+=n
                if tid%31==27:continue
                if tid==17:
                    versions=[int.from_bytes(val[i:i+4],'big') for i in range(4,len(val),4)]
                    v[str(tid)]={'chosen':int.from_bytes(val[:4],'big'),'available':sorted(x for x in versions if x&0x0f0f0f0f!=0x0a0a0a0a)}
                elif tid==0x3127:
                    rtt,end=varint(val,0)
                    if end!=len(val) or not 0<rtt<60_000_000:raise ValueError('invalid measured RTT')
                    v[str(tid)]='measured RTT (microseconds)'
                else:v[str(tid)]=val.hex()
        elif ident==41:v='session-specific PSK'
        result['extensions'][str(ident)]=v
    return result

def controls(path):
    streams={}
    for line in pathlib.Path(path).read_text().splitlines():
        row=json.loads(line)
        if 'controlStream' in row:
            streams.setdefault((row['peer'],row['controlStream']),bytearray()).extend(base64.b64decode(row['bytes']))
    result=[]
    for data in streams.values():
        kind,p=varint(data,0)
        if kind!=0:continue
        frames=[];settings=[];priorities=[]
        while p<len(data):
            typ,p=varint(data,p);n,p=varint(data,p);payload=data[p:p+n];p+=n
            if len(payload)!=n:raise ValueError('truncated control frame')
            if typ==4:
                frames.append('SETTINGS');j=0
                while j<len(payload):
                    key,j=varint(payload,j);value,j=varint(payload,j)
                    settings.append(['GREASE','random'] if key%31==2 else [key,value])
            elif typ%31==2:
                if typ<33 or len(payload)!=(typ-33)//31%4:raise ValueError('unexpected GREASE payload length')
                frames.append('GREASE')
            elif typ==0xf0700:
                frames.append('PRIORITY_UPDATE');sid,j=varint(payload,0)
                priorities.append([sid,payload[j:].decode('ascii')])
            else:raise ValueError(f'unexpected control frame {typ}')
        if frames[:2]!=['SETTINGS','GREASE'] or any(f!='PRIORITY_UPDATE' for f in frames[2:]):
            raise ValueError(f'unexpected control order {frames}')
        result.append({'settings':settings,'frames':frames,'priorities':priorities})
    if not result:raise ValueError('missing control stream capture; use --raw-h3')
    return result

def main(path):
    path=pathlib.Path(path)
    chrome=decode(path/'chrome-wire.jsonl');mimic=decode(path/'mimic-wire.jsonl')
    a,b=normalize(chrome[0]),normalize(mimic[0])
    diffs={k:{'chrome':a['extensions'].get(k),'mimic':b['extensions'].get(k)} for k in a['extensions'].keys()|b['extensions'].keys() if a['extensions'].get(k)!=b['extensions'].get(k)}
    resumed_chrome=next(h for h in chrome if any(e['id']==41 for e in h['extensions']))
    resumed_mimic=next(h for h in mimic if any(e['id']==41 for e in h['extensions']))
    ra,rb=normalize(resumed_chrome),normalize(resumed_mimic)
    resumed_diffs={k:{'chrome':ra['extensions'].get(k),'mimic':rb['extensions'].get(k)} for k in ra['extensions'].keys()|rb['extensions'].keys() if ra['extensions'].get(k)!=rb['extensions'].get(k)}
    cc,mc=controls(path/'chrome-wire.jsonl'),controls(path/'mimic-wire.jsonl')
    settings_match=all(c['settings']==cc[0]['settings'] for c in cc+mc)
    result={'chromeJA4':ja4(chrome[0],'q'),'chromeResumedJA4':ja4(resumed_chrome,'q'),'mimicJA4':[ja4(h,'q') for h in mimic],'extensionDifferences':diffs,'resumedExtensionDifferences':resumed_diffs,'settingsMatch':settings_match,'chromeControl':cc,'mimicControl':mc,'chrome':a,'mimic':b}
    print(json.dumps(result,indent=2))
    return bool(diffs or resumed_diffs or not settings_match or a['ciphers']!=b['ciphers'] or a['sessionIDLength']!=b['sessionIDLength'] or ra['ciphers']!=rb['ciphers'] or ra['sessionIDLength']!=rb['sessionIDLength'])
if __name__=='__main__':sys.exit(main(sys.argv[1]))
