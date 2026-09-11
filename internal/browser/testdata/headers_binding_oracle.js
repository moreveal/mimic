(() => {const all=(() => {
  const out = {}, capture = (name, fn) => { try { out[name] = {value: fn()}; } catch(e) { out[name] = {error: e.name}; } };
  for (const method of ['append','set','get','has','delete','entries','keys','values','forEach']) {
    let log = [];
    const arg = {toString(){log.push('convert'); return 'x';}};
    capture('brand.'+method, () => { Headers.prototype[method].call({}, arg, arg); return 'returned'; });
    out['brandLog.'+method] = log;
  }
  for (const method of ['append','set','get','has','delete']) capture('arity.'+method, () => { const h = new Headers(); h[method](); return [...h]; });
  for (const [name,value] of [['spaced-name',' x '],['symbol-name',Symbol('x')],['unicode-name','\u0100']]) capture(name, () => new Headers([[value,'ok']]).get('x'));
  for (const [name,value] of [['symbol-value',Symbol('x')],['unicode-value','\u0100'],['latin-value','\u00a0x\u00a0'],['vertical-value','\vx\v']]) capture(name, () => new Headers([['x',value]]).get('x'));
  for (const [name,init] of [['short',[['x']]],['long',[['x','a','b']]],['primitive',1],['null',null]]) capture('init.'+name, () => [...new Headers(init)]);
  capture('init.override', () => { class H extends Headers { append(){throw Error('override');} } return [...new H([['x','a']])]; });
  for (const method of ['entries','keys','values']) capture('live.'+method, () => {const h=new Headers([['a','1'],['c','3']]),i=h[method]();const first=i.next();h.set('b','2');h.set('c','4');return [first,i.next(),i.next(),i.next()];});
  capture('forEach.live', () => {const h=new Headers([['a','1'],['c','3']]),seen=[];h.forEach((v,k)=>{seen.push([k,v]);if(k==='a'){h.set('b','2');h.set('c','4');}});return seen;});
  capture('forEach.callback', () => {new Headers().forEach(null);return 'returned';});
  return out;
})()
;return Object.fromEntries(Object.entries(all).filter(([key])=>/^(brand|brandLog|arity)\.|^forEach.callback$/.test(key)));})()