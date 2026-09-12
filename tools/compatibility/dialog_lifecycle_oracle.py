"""Measure dialog lifecycle in owned Chrome 152 and Mimic contexts."""
import argparse
import asyncio
import json
from pathlib import Path
from locale_oracle import capture

async def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--chrome',type=int,default=9351)
    parser.add_argument('--mimic',type=int)
    parser.add_argument('--output',type=Path,required=True)
    args=parser.parse_args()
    source=(Path(__file__).resolve().parents[2]/'internal/browser/testdata/dialog_lifecycle_oracle.js').read_text(encoding='utf-8')
    reference=await capture(args.chrome,False,source,[('UTC','en-US')])
    observations=reference['results']['UTC']
    args.output.write_text(json.dumps(observations,indent=2)+'\n',encoding='utf-8')
    if args.mimic:
        mimic=await capture(args.mimic,True,source,[('UTC','en-US')])
        actual=mimic['results']['UTC']
        if actual!=observations:
            print('EXPECTED',json.dumps(observations))
            print('ACTUAL',json.dumps(actual))
            raise AssertionError('Dialog lifecycle differs')
        print('Dialog lifecycle matches',reference['browser'])

if __name__=='__main__':
    asyncio.run(main())
