"""Bounded browser-semantic probes; no runtime dependencies or site fixtures.

Expressions return observation records. Each case runs in a fresh CDP target.
Only explicitly allowlisted methods are invoked; discovering an API never
implies that invoking it (navigation, dialogs, devices, network) is safe.
"""
import json
from itertools import zip_longest

ROOTS = ("window", "document", "navigator", "performance", "screen", "history", "location")

HELPERS = r"""
const tag = v => Object.prototype.toString.call(v);
const error = e => {
 const message = String(e && e.message || e);
 let category = /illegal invocation|incompatible receiver|not of type|called on|does not implement/i.test(message) ? 'illegal-receiver' :
 /denied|cross-origin|blocked a frame/i.test(message) ? 'security' :
 /argument|parameter|convert|Symbol/i.test(message) ? 'argument' :
 /not defined/i.test(message) ? 'not-defined' :
 /not a function/i.test(message) ? 'not-callable' : 'other';
 return {exception: String(e && e.name || 'Error'), class: tag(e), messageCategory:category};
};
const scalar = v => {
 if (v === undefined) return {type:'undefined'};
 if (v === null) return {type:'object',value:null};
 const t = typeof v;
 if (t === 'number') return {type:t,value:Number.isNaN(v)?'NaN':v===Infinity?'Infinity':v===-Infinity?'-Infinity':Object.is(v,-0)?'-0':v};
 if (t === 'symbol' || t === 'bigint') return {type:t,value:String(v)};
 if (t === 'string' || t === 'boolean') return {type:t,value:v};
 return {type:t,tag:tag(v)};
};
const attempt = fn => {try{return {ok:scalar(fn())};}catch(e){return error(e);}};
const key = k => typeof k === 'symbol' ? {symbol:String(k)} : k;
const inspect = v => ({
 type:typeof v, strictUndefined:v===undefined, looseUndefined:v==undefined,
 strictNull:v===null, looseNull:v==null, boolean:Boolean(v),
 primitive:scalar(v), tag:attempt(()=>tag(v)),
 prototype:attempt(()=>Object.getPrototypeOf(v)),
 prototypeIsConstructorPrototype:attempt(()=>Object.getPrototypeOf(v)===v.constructor.prototype),
 constructor:attempt(()=>v.constructor), constructorName:attempt(()=>v.constructor.name),
 constructorIsGlobal:attempt(()=>v.constructor===globalThis[v.constructor.name]),
 ownKeys:(()=>{try{return Reflect.ownKeys(v).map(key);}catch(e){return error(e);}})(),
 functionShape:typeof v==='function'?{name:attempt(()=>v.name),length:attempt(()=>v.length),source:attempt(()=>Function.prototype.toString.call(v))}:null
});
const descriptor = (o,k) => {
 let depth=0;
 for(let p=o;p!==null;p=Object.getPrototypeOf(p),depth++){
  const d=Object.getOwnPropertyDescriptor(p,k);
  if(d){const result={depth,configurable:d.configurable,enumerable:d.enumerable};
   if('value' in d){result.kind='data';result.writable=d.writable;result.value=scalar(d.value);}
   else {result.kind='accessor';result.get=scalar(d.get);result.set=scalar(d.set);}
   return result;
  }
 }
 return null;
};
"""


def render_probe(probe):
    """Return standalone JS: helpers + removable setup statements + observation."""
    return "(async () => {\n" + HELPERS + "\ntry {\n" + "\n".join(probe.get("setup", [])) + "\nreturn {status:'ok',value:await (" + probe["expression"] + ")};\n} catch(e) {return {status:'exception',error:error(e)};}\n})()"


def discovery_expression():
    # Avoid getter execution during discovery. Preserve symbols in ownKeys
    # observations; string properties are sufficient for this first walker.
    return "(() => {const out={};for(const name of " + json.dumps(ROOTS) + "){const keys=new Set();for(let p=globalThis[name];p!==null;p=Object.getPrototypeOf(p)){for(const k of Reflect.ownKeys(p))if(typeof k==='string')keys.add(k);}out[name]=Array.from(keys).sort();}return out;})()"


def probe(id, category, expression, setup=()):
    return {"id": id, "category": category, "setup": list(setup), "expression": expression}


def surface_probes(discovery):
    """Inspect all discovered properties without invoking discovered methods."""
    result = [probe("surface." + root, "surface", "inspect(" + root + ")") for root in ROOTS]
    per_root = []
    for root in ROOTS:
        properties = []
        for name in sorted(set(discovery.get(root, []))):
            if not isinstance(name, str):
                continue
            if (root, name) in {("performance", "timeOrigin"), ("performance", "timing"), ("performance", "memory")} :
                continue  # Raw clock/process counters are not differential invariants.
            access = root + "[" + json.dumps(name) + "]"
            # Separate descriptor and value observations to aid minimization and grouping.
            properties.append(probe("descriptor." + root + "." + name, "descriptor", "descriptor(" + root + "," + json.dumps(name) + ")"))
            properties.append(probe("property." + root + "." + name, "surface", "inspect(" + access + ")"))
        per_root.append(properties)
    for row in zip_longest(*per_root):
        result.extend(item for item in row if item is not None)
    return result


FRAME = ["const frame=document.createElement('iframe');", "document.body.appendChild(frame);", "const other=frame.contentWindow;"]


def generate_probes():
    cases = [
        probe("legacy.document-all", "legacy", "inspect(document.all)"),
        probe("legacy.document-all-descriptor", "descriptor", "({all:descriptor(document,'all'),name:descriptor(document.all,'name'),length:descriptor(document.all,'length'),prototype:attempt(()=>Object.getPrototypeOf(document.all)===HTMLAllCollection.prototype),constructor:attempt(()=>document.all.constructor===HTMLAllCollection),source:attempt(()=>Function.prototype.toString.call(document.all))})"),
        probe("realm.window-proxy-navigation", "realm", "({proxy:saved===frame.contentWindow,documentChanged:savedDocument!==frame.contentDocument,parent:frame.contentWindow.parent===window})", FRAME + ["const saved=other;const savedDocument=other.document;", "await new Promise((resolve,reject)=>{frame.onload=resolve;frame.onerror=()=>reject(new Error('iframe navigation failed'));frame.src='about:blank?compat_navigation';});"]),
        probe("legacy.document-all-call", "legacy", "({callable:attempt(()=>document.all('compat_named')===document.getElementById('compat_named')), indexed:attempt(()=>document.all[0]===document.documentElement),missing:attempt(()=>document.all('compat_missing'))})", ["const item=document.createElement('div');item.id='compat_named';document.body.appendChild(item);"]),
        probe("legacy.window-named", "legacy", "({windowNamed:window.compat_named===item,documentNamed:document.compat_named===item,descriptor:descriptor(window,'compat_named')})", ["const item=document.createElement('form');item.name='compat_named';document.body.appendChild(item);"]),
        probe("legacy.collection-indexed-named", "legacy", "({collection:inspect(nodes),index:nodes[0]===item,named:nodes.compat_named===item,namedItem:nodes.namedItem('compat_named')===item,missing:attempt(()=>nodes.item(9)),listIndex:document.querySelectorAll('form')[0]===item,listMissing:attempt(()=>document.querySelectorAll('form').item(9))})", ["const item=document.createElement('form');item.id='compat_named';document.body.appendChild(item);", "const nodes=document.forms;"]),
        probe("legacy.live-collection", "legacy", "({liveBefore,staticBefore,liveAfter:live.length,staticAfter:fixed.length,identity:live===document.getElementsByTagName('span')})", ["const live=document.getElementsByTagName('span');const fixed=document.querySelectorAll('span');const liveBefore=live.length;const staticBefore=fixed.length;", "document.body.appendChild(document.createElement('span'));"]),
        probe("realm.main-window", "realm", "({tag:tag(window),instance:window instanceof Window,prototype:Object.getPrototypeOf(window)===Window.prototype,constructor:window.constructor===Window,self:window===globalThis})"),
        probe("realm.same-origin", "realm", "({differentObject:other.Object!==Object,sameProxy:other===frame.contentWindow,frameElement:other.frameElement===frame,parent:other.parent===window,document:other.document===frame.contentDocument,instance:other.document instanceof other.Document,notMainInstance:other.document instanceof Document})", FRAME),
        probe("realm.inside-iframe", "realm", "other.eval('({self:window===globalThis,parentDifferent:parent!==window,ownObject:Object===window.Object,documentBrand:Object.prototype.toString.call(document)})')", FRAME),
        probe("realm.detached-iframe", "realm", "({proxy:other===saved,eval:attempt(()=>savedEval('6*7')),document:attempt(()=>saved.document===savedDocument),frameWindow:attempt(()=>frame.contentWindow===saved)})", FRAME + ["const saved=other;const savedEval=other.eval;const savedDocument=other.document;", "frame.remove();"]),
        probe("realm.about-blank-inheritance", "realm", "({accessible:other.document===frame.contentDocument,url:other.location.href,parentOrigin:other.parent.location.origin===location.origin,origin:other.origin===window.origin})", FRAME),
        probe("realm.cross-origin", "realm", "({document:attempt(()=>other.document),location:attempt(()=>other.location.href),postMessage:attempt(()=>typeof other.postMessage),self:attempt(()=>other===frame.contentWindow),tag:attempt(()=>tag(other))})", ["const frame=document.createElement('iframe');frame.src='__FUZZ_CROSS_ORIGIN__/';", "await new Promise((resolve,reject)=>{frame.onload=resolve;frame.onerror=()=>reject(new Error('iframe load failed'));document.body.appendChild(frame);});", "const other=frame.contentWindow;"]),
        probe("timing.jobs", "timing", "order", ["const order=[];", "queueMicrotask(()=>order.push('microtask'));Promise.resolve().then(()=>order.push('promise'));", "await new Promise(resolve=>setTimeout(()=>{order.push('timeout');resolve();},0));"]),
        probe("timing.mutation-observer", "timing", "order", ["const order=[];const node=document.createElement('div');", "const observer=new MutationObserver(()=>order.push('observer'));observer.observe(node,{attributes:true});", "Promise.resolve().then(()=>order.push('promise-before'));node.setAttribute('data-x','1');queueMicrotask(()=>order.push('microtask-after'));", "await new Promise(resolve=>setTimeout(resolve,0));observer.disconnect();"]),
        probe("timing.performance-monotonic", "timing", "({finite:values.every(Number.isFinite),nonnegative:values.every(v=>v>=0),monotonic:values.every((v,i)=>i===0||v>=values[i-1])})", ["const values=Array.from({length:128},()=>performance.now());"]),
    ]
    # Pure queries only: no random/time return values, mutation, dialogs, network,
    # navigation, permissions, or callback-taking methods.
    methods = [("document", "hasFocus"), ("document", "getElementById"), ("document", "querySelector"), ("navigator", "javaEnabled"), ("performance", "getEntriesByName")]
    arguments = [("missing", ""), ("undefined", "undefined"), ("null", "null"), ("primitive", "'compat_missing'"), ("object", "{}"), ("symbol", "Symbol('compat')")]
    for root, method in methods:
        for receiver, value in [("correct", root), ("object", "{}"), ("null", "null"), ("foreign", "new other.Object()")]:
            for label, arg in arguments:
                cases.append(probe("call." + root + "." + method + "." + receiver + "." + label, "brand-check", "attempt(()=>Reflect.apply(" + root + "." + method + "," + value + ",[" + arg + "]))", FRAME if receiver == "foreign" else []))
    return cases
