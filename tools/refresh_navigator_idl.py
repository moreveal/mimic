"""Repair Navigator provenance from the pinned sources, retaining raw inputs."""
import base64
import concurrent.futures
import json
from pathlib import Path
import generate_compat as g

def main():
    target=json.loads((g.ROOT/'chrome/152/target.json').read_text())
    output=g.ROOT/target['generated_dir']
    catalog=json.loads((output/'webapi.json').read_text())
    nav=next(d for d in catalog['declarations'] if d['name']=='Navigator')
    paths={nav['source'],*nav['partials']}
    types={m.get('type') for m in nav.get('members',[]) if m.get('kind')=='attribute'}
    types.update(['PermissionStatus','MediaDeviceInfo','StorageBucket','Lock','DelegatedInkTrailPresenter'])
    for d in catalog['declarations']:
        if d['name'] in nav['includes'] or d['name'] in types:
            paths.add(d['source']);paths.update(d.get('partials',[]))
    retained=output/'navigator-idl-sources.json'
    sources=json.loads(retained.read_text())['sources'] if retained.exists() else {}
    missing=paths-sources.keys()
    if missing:
        def fetch(path):
            return path,base64.b64decode(g.fetch(g.GITILES.format(ref=target['chromium_commit'],path=path)+'?format=TEXT')).decode()
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
            sources.update(pool.map(fetch,sorted(missing)))
        g.write_utf8(retained,json.dumps({'chromiumCommit':target['chromium_commit'],'sources':sources},indent=2)+'\n')
    corrected=g.normalize_idl(sources,target)
    replacements={d['name']:d for d in corrected['declarations']}
    catalog['declarations']=[replacements.get(d['name'],d) for d in catalog['declarations']]
    g.write_utf8(output/'webapi.json',json.dumps(catalog,ensure_ascii=False,indent=2)+'\n')
    g.write_utf8(output/'surface.js',g.generate_surface_js(catalog))
    print(f'Recovered member provenance from {len(sources)} pinned sources')

if __name__=='__main__':main()
