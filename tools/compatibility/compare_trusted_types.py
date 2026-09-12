"""Compare retained TT observations; preserve semantic TT/CSP diagnostics."""
import argparse
from copy import deepcopy
import hashlib
import json
from pathlib import Path


def normalized(want, got):
    want, got = deepcopy(want), deepcopy(got)
    if isinstance(want, dict):
        message = want.get('message', '')
        if 'This document requires' not in message and 'Evaluating a string as JavaScript violates' not in message:
            want.pop('message', None)
            if isinstance(got, dict):
                got.pop('message', None)
    return want, got


def compare(oracle, actual):
    differences, unsupported = {}, {}
    rows = 0
    for case, expected in oracle['cases'].items():
        observed = actual['cases'].get(case, {})
        for name, want in expected.items():
            rows += 1
            got = observed.get(name)
            left, right = normalized(want, got)
            if left != right:
                differences.setdefault(case, []).append(name)
                if name == 'SharedWorker' and isinstance(want, dict) and not want.get('error') and isinstance(got, dict) and got.get('error') == 'NotSupportedError' and got.get('calls') == want.get('calls'):
                    unsupported.setdefault(case, []).append(name)
    return {'observations': rows, 'different': sum(map(len, differences.values())), 'explicitUnsupported': sum(map(len, unsupported.values())), 'differencesByCase': differences, 'unsupportedByCase': unsupported}


def mask(capture):
    case = capture['cases']['required:sinks:default']
    return sum(bit for name, bit in [('innerHTML', 1), ('eval', 2), ('scriptSrc', 4)] if case[name].get('calls'))


def main(args):
    paths = [Path(args.oracle), Path(args.before), Path(args.after)]
    oracle, before, after = [json.loads(p.read_text()) for p in paths]
    result = {'reference': oracle['captureMetadata'], 'baselineHead': args.baseline_head, 'inputSHA256': {k: hashlib.sha256(p.read_bytes()).hexdigest() for k, p in zip(['oracle','before','after'],paths)}, 'before': compare(oracle,before), 'after': compare(oracle,after), 'defaultPolicyMask': {'chrome':mask(oracle),'before':mask(before),'after':mask(after)}}
    result['examples'] = {case+':'+name: {'chrome':oracle['cases'][case][name], 'before':before['cases'][case].get(name), 'after':after['cases'][case].get(name)} for case,name in [('required:sinks:plain','eval'),('required:sinks:plain','Function'),('required:sinks:plain','innerHTML'),('required:sinks:plain','scriptSrc'),('required:sinks:default','eval'),('required:sinks:default','innerHTML'),('required:sinks:default','scriptSrc'),('required:execution','noDefault:textNode'),('required:execution','default:connectedTextNode'),('required:realms','parentDefaultDoesNotSupplyChild'),('open:lifecycle','newBlankInherits')]}
    Path(args.output).write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({k:result[k] for k in ['defaultPolicyMask']}))
    for name in ['before','after']:
        print(name, {k:v for k,v in result[name].items() if not k.endswith('ByCase')})


if __name__ == '__main__':
    p=argparse.ArgumentParser()
    p.add_argument('--oracle',default='internal/browser/testdata/trusted_types_chrome152.json')
    p.add_argument('--before',required=True)
    p.add_argument('--after',required=True)
    p.add_argument('--baseline-head',required=True)
    p.add_argument('--output',required=True)
    main(p.parse_args())
