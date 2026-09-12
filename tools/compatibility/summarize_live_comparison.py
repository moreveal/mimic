"""Build auditable CSV/JSON summaries from live_browser_comparison receipts."""
import argparse
import csv
import json
import re
from pathlib import Path
import statistics


def summarize(path):
    d = json.loads(path.read_text(encoding='utf-8'))
    events = d.get('events', [])
    final = d.get('final') or {}
    responses = [e['params'] for e in events if e.get('method')=='Network.responseReceived']
    documents = [r['response'] for r in responses if r.get('type')=='Document']
    top = [r for r in documents if r.get('url','').split('#')[0]==final.get('url','').split('#')[0]]
    main = top[-1] if top else documents[0] if documents else {}
    headers = {k.lower():v for k,v in main.get('headers',{}).items()}
    exceptions = [e['params'].get('exceptionDetails',{}) for e in events if e.get('method')=='Runtime.exceptionThrown']
    failures = [e['params'] for e in events if e.get('method')=='Network.loadingFailed']
    cf_challenges = [r for r in documents if r.get('url','').split('#')[0] in (d['url'].split('#')[0],final.get('url','').split('#')[0])
        and str({k.lower():v for k,v in r.get('headers',{}).items()}.get('cf-mitigated','')).lower()=='challenge']
    record = {k:d.get(k) for k in ['site','url','config','repetition','outcome','first_content_ms','dcl_ms','load_ms','cdp_ready_ms','observation_ms','challenge_seen','error']}
    record.update(status=main.get('status'),server=headers.get('server'),cf_ray=bool(headers.get('cf-ray')),
        cf_mitigated=headers.get('cf-mitigated'),cf_challenge_documents=len(cf_challenges),
        title=final.get('title'),final_url=final.get('url'),text_length=final.get('textLength'),
        selector_count=final.get('selectorCount'),links=final.get('links'),frames=final.get('frames'),
        requests=sum(e.get('method')=='Network.requestWillBeSent' for e in events),responses=len(responses),
        failed_requests=len(failures),js_exceptions=len(exceptions),probe_errors=len(d.get('probeErrors',[])),
        encoded_bytes=sum(e['params'].get('encodedDataLength',0) for e in events if e.get('method')=='Network.loadingFinished'),
        evidence=str(path.resolve()))
    record['error_examples'] = [x.get('exception',{}).get('description',x.get('text',''))[:500] for x in exceptions[:5]]
    record['network_error_examples'] = [x.get('errorText','') for x in failures[:5]]
    record['text_excerpt'] = final.get('text','')[:1000]
    record['audited_outcome'] = record['outcome']
    record['audited_content_ms'] = record['first_content_ms']
    record['audit_note'] = ''
    if record['outcome']=='content_not_confirmed' and record['first_content_ms'] is not None and record['status']==200 and not final.get('challenge') and final.get('selectorCount',0)>0 and final.get('textLength',0)>=d['minimumTextLength']:
        record['audited_outcome']='content_seen_not_reconfirmed'
        record['audit_note']='A valid content sample was obtained, but the second confirmation at least three seconds later did not complete before the observation deadline.'
    if d['site']=='cf-lab' and 'You bypassed the Cloudflare challenge!' in final.get('text','') and record['status']==200:
        valid=[s for s in d.get('samples',[]) if 'You bypassed the Cloudflare challenge!' in s.get('text','')]
        record['audited_outcome']='content_after_challenge' if cf_challenges else 'content_observed'
        record['audited_content_ms']=valid[0]['elapsed_ms'] if valid else None
        if record['outcome'] not in ('content_observed','content_after_challenge'):
            record['audit_note']='Exact success message and final HTTP 200. Initial text-length threshold was too large for this short success page.'
    if d['site'] in ('amiibo','lowendbox','spigotmc') and record['outcome']=='content_not_confirmed':
        # Exploratory selector errors verified on Chrome and every saved DOM.
        # Preserve original outcomes, timestamps and snapshots unchanged.
        def matches(s):
            if s.get('challenge'):
                return False
            if d['site']=='amiibo':
                return s.get('links',0)>=10 and 'Animal Crossing' in s.get('text','') and 'Sandy' in s.get('text','')
            if d['site']=='spigotmc':
                return s.get('links',0)>=20 and s.get('textLength',0)>=1000 and 'SpigotMC' in s.get('title','') and 'BuildTools' in s.get('text','')
            return s.get('links',0)>=20 and s.get('textLength',0)>=1000 and 'LowEndBox' in s.get('title','') and 'Hosting' in s.get('text','')
        valid = [s for s in d.get('samples',[]) if matches(s)]
        if valid and matches(final) and record['status']==200:
            record['audited_outcome']='content_observed'
            record['audited_content_ms']=valid[0]['elapsed_ms']
            record['audit_note']='Initial exploratory criterion false negative; corrected uniformly using retained content. Original result unmodified.'
    if record['cf_mitigated']=='challenge':
        record['audited_outcome']='challenge_unresolved'
        record['audited_content_ms']=None
        if record['outcome']=='http_error':
            record['audit_note']='Localized challenge missed by initial text detector; early stop. Use corrected repeat for timeout conclusions.'
    record['challenge_exit_ms']=None
    if cf_challenges and record['cf_mitigated']!='challenge' and record['status'] in (200,404):
        transitions=[e for e in events if e.get('method')=='Network.responseReceived'
            and e['params'].get('type')=='Document' and e['params']['response'].get('url')==final.get('url')
            and e['params']['response'].get('status')==record['status']]
        if transitions:
            record['challenge_exit_ms']=transitions[-1]['elapsed_ms']
            if record['status']==404:
                record['audited_outcome']='origin_404_after_challenge'
                record['audit_note']='Challenge 403 followed by application 404 at same URL; interstitial cleared, requested page unavailable.'
    record['interaction']=d.get('interaction')
    verdict=re.search(r'Test Results:\s*(Robot|Normal|Human|No bots detected)',final.get('text','')) if d['site']=='browserscan' else None
    record['bot_detection_verdict']=verdict.group(1) if verdict else None
    return record


def main():
    p = argparse.ArgumentParser()
    p.add_argument('root',type=Path)
    args = p.parse_args()
    records = [summarize(f) for f in sorted(args.root.glob('*/result.json'))]
    (args.root/'summary.json').write_text(json.dumps(records,ensure_ascii=False,indent=2),encoding='utf-8')
    if records:
        with (args.root/'summary.csv').open('w',encoding='utf-8-sig',newline='') as f:
            w=csv.DictWriter(f,fieldnames=list(records[0]))
            w.writeheader(); w.writerows(records)
    for r in records:
        print(json.dumps({k:r[k] for k in ['site','config','repetition','outcome','first_content_ms','status','cf_ray','cf_mitigated','js_exceptions','probe_errors','title']},ensure_ascii=False))


if __name__=='__main__':
    main()
