"""Retain only loopback ClientHello evidence, without unrelated NetLog traffic."""
import argparse
import base64
import hashlib
import json
import pathlib
from decode import decode, hello
from compare import ja4, normalize, controls

p=argparse.ArgumentParser()
p.add_argument('--quic-dir',required=True)
p.add_argument('--tcp-dir',required=True)
p.add_argument('--chrome',required=True)
p.add_argument('--output',required=True)
a=p.parse_args()
q=pathlib.Path(a.quic_dir);tcp=pathlib.Path(a.tcp_dir)
netlog=json.loads((tcp/'netlog.json').read_text())
tls=[]
for e in netlog['events']:
    if e['type']==netlog['constants']['logEventTypes']['SSL_HANDSHAKE_MESSAGE_SENT'] and e.get('params',{}).get('type')==1:
        h=hello(base64.b64decode(e['params']['bytes']))
        if any(x['id']==0 and b'localhost' in bytes.fromhex(x['data']) for x in h['extensions']):tls.append(h)
quic=decode(q/'chrome-wire.jsonl')
result={'reference':{'product':'Chrome/152.0.7977.82','platform':'windows-x64','mode':'headful','profile':'fresh per capture','origin':json.loads((q/'metadata.json').read_text())['origin'],'chromeSHA256':hashlib.sha256(pathlib.Path(a.chrome).read_bytes()).hexdigest(),'quicMetadata':json.loads((q/'metadata.json').read_text()),'tcpMetadata':json.loads((tcp/'metadata.json').read_text()),'limitations':['Loopback certificate is temporarily trusted in CurrentUser Root.','QUIC uses origin-to-force-quic-on because Chrome rejects local roots for ordinary QUIC.','Volatile key material, PSK identities/binders, extension permutations and GREASE values are normalized.','No claims about loss, migration, proxies or arbitrary feature/experiment configurations.']}}
meta=result['reference']['quicMetadata']
result['http3Control']=controls(q/'chrome-wire.jsonl')
result['captureMetadata']={'schemaVersion':1,'chromeVersion':'152.0.7977.82','chromiumRevision':1669021,'chromiumCommit':'d04cdb24d67b081f6cf80200ffc5233f44b61109','v8Version':meta['browserVersion']['jsVersion'],'platform':'windows-x64','browserMode':'headful','commandLineFeatureOverrides':[x for x in meta['flags'] if x.startswith('--origin-to-force-quic-on')],'environmentProfileId':'chrome-152-windows-x64-headful-loopback-wire-v1','profileFreshness':'fresh-controlled',**meta['observations']}
for name,hs,protocol in [('tls',tls,'t'),('quic',quic,'q')]:
    result[name]=[]
    for resumed in [False,True]:
        h=next(h for h in hs if any(e['id']==41 for e in h['extensions'])==resumed)
        result[name].append({'resumed':resumed,'ja4':ja4(h,protocol),'hello':normalize(h)})
pathlib.Path(a.output).write_text(json.dumps(result,indent=2)+'\n')
