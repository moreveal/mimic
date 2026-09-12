"""Generate Performance binding shape from the two controlled Chrome .82 oracles."""
import argparse
import hashlib
import json
from pathlib import Path

root=Path(__file__).resolve().parents[1]
fixtures=root/'internal/browser/testdata'
window=json.loads((fixtures/'performance_surface_chrome152.json').read_text())['result']['result']['value']
worker=json.loads((fixtures/'performance_surface_worker_chrome152.json').read_text())['result']['result']['value']
schema={}
for name,value in window.items():
    if not isinstance(value,dict) or 'members' not in value:continue
    other=worker.get(name)
    schema[name]={'length':value['length'],'parent':value['parent'],'worker':isinstance(other,dict),'members':value['members']}
    if isinstance(other,dict):schema[name]['workerMembers']=other['members']
encoded=json.dumps(schema,separators=(',',':'))+'\n'
target=root/'internal/webapi/performance_interfaces.json'
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--check',action='store_true')
args=parser.parse_args()
if args.check:
    if target.read_text()!=encoded:raise SystemExit('Performance interface schema is stale')
    captures=[path for path in fixtures.glob('performance_*_chrome152.json') if 'fixture' in json.loads(path.read_text())]
    for path in captures:
        data=json.loads(path.read_text())
        if data['captureMetadata']['chromeVersion']!='152.0.7977.82':raise SystemExit(f'{path.name}: wrong Chrome version')
        source=root/data['fixture']
        digest=hashlib.sha256(source.read_text(encoding='utf8').encode()).hexdigest()
        if digest!=data['fixtureSHA256']:raise SystemExit(f'{path.name}: fixture changed after capture')
        if 'captureError' in data['result']['result']['value']:raise SystemExit(f'{path.name}: failed capture')
    print(f'PASS: {len(schema)} interface shapes; {len(captures)} exact .82 captures and fixture hashes')
else:
    target.write_text(encoded)
