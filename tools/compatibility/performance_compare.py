"""Report retained Chrome relations versus two independently built Mimic runs."""
import argparse
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]

def value(path):
    return json.loads(path.read_text(encoding='utf8'))['result']['result']['value']

def differences(expected,actual,path=''):
    if type(expected)!=type(actual):return [{'path':path,'expected':expected,'actual':actual}]
    if isinstance(expected,dict):
        result=[]
        for key in sorted(expected.keys()|actual.keys()):
            if key not in expected or key not in actual:
                result.append({'path':path+'.'+key,'expected':expected.get(key),'actual':actual.get(key)})
            else:result+=differences(expected[key],actual[key],path+'.'+key)
        return result
    if isinstance(expected,list):
        if len(expected)!=len(actual):return [{'path':path,'expected':expected,'actual':actual}]
        return [d for i,(e,a) in enumerate(zip(expected,actual)) for d in differences(e,a,f'{path}[{i}]')]
    if expected!=actual:return [{'path':path,'expected':expected,'actual':actual}]
    return []

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--before',type=Path,required=True)
    parser.add_argument('--after',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True)
    args=parser.parse_args();rows=[]
    for golden in sorted((ROOT/'internal/browser/testdata').glob('performance_*_chrome152.json')):
        name=golden.stem.removesuffix('_chrome152')
        if 'fixture' not in json.loads(golden.read_text()):continue
        row={'fixture':name,'reference':str(golden.relative_to(ROOT)).replace('\\','/')}
        for phase,folder in [('before',args.before),('after',args.after)]:
            path=folder/(name+'_mimic.json')
            if not path.exists():row[phase]={'status':'MISSING'};continue
            actual=value(path)
            if 'captureError' in actual:
                row[phase]={'status':'CAPTURE_ERROR','error':actual['captureError']}
                continue
            delta=differences(value(golden),actual)
            row[phase]={'status':'MATCH' if not delta else 'DIFFERENT','differences':delta}
        rows.append(row)
    output={'reference':'Chrome/152.0.7977.82','comparison':'Exact equality of controlled structural results and relations; no absolute wall-clock or heap-byte matching.','fixtures':rows}
    args.output.parent.mkdir(parents=True,exist_ok=True)
    args.output.write_text(json.dumps(output,ensure_ascii=False,indent=2)+'\n',encoding='utf8')
    for phase in ['before','after']:
        print(phase,{status:sum(r[phase]['status']==status for r in rows) for status in ['MATCH','DIFFERENT','CAPTURE_ERROR','MISSING']})

if __name__=='__main__':main()
