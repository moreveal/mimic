import {build} from 'esbuild';
import {writeFileSync,readFileSync} from 'node:fs';
import {resolve} from 'node:path';
const result=await build({metafile:true,entryPoints:['entry.js'],bundle:true,format:'iife',globalName:'mimicSelectorLibrary',minify:true,legalComments:'none',outfile:'../../selectors_vendor.js',plugins:[{name:'export-attribute-policy',setup(build){build.onLoad({filter:/css-select[\\/]dist[\\/]esm[\\/]attributes\.js$/},args=>({contents:readFileSync(args.path,'utf8')+'\nexport {caseInsensitiveAttributes};',loader:'js'}));}},{name:'selector-parser-only',setup(build){build.onResolve({filter:/^css-tree$/},()=>({path:resolve('css-tree-small.js')}));}}]});

writeFileSync('bundle-manifest.json',JSON.stringify({inputs:Object.fromEntries(Object.entries(result.metafile.outputs['../../selectors_vendor.js'].inputs).filter(([,v])=>v.bytesInOutput>0).map(([k,v])=>[k,v.bytesInOutput]))},null,2)+'\n');
