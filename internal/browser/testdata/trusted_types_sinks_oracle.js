(() => {
  const mode = "$MODE", out = {}, calls = [];
  const marker = {marker: true};
  const p = trustedTypes.createPolicy('allowed', {createHTML:s=>s, createScript:s=>s, createScriptURL:s=>s});
  if (['default','null','throw'].includes(mode)) {
    const options = {};
    for (const method of ['createHTML','createScript','createScriptURL']) options[method] = function(...args) {
      calls.push([method, ...args]);
      if (mode === 'throw') throw marker;
      return mode === 'null' ? null : args[0];
    };
    trustedTypes.createPolicy('default', options);
  }
  const value = (kind, text) => mode === 'trusted' ? p[kind](text) : mode === 'wrong' ? p[kind === 'createHTML' ? 'createScript' : 'createHTML'](text) : text;
  const html = s => value('createHTML',s), script = s => value('createScript',s), url = s => value('createScriptURL',s);
  const box = document.querySelector('#box'), elem = () => document.createElement('script');
  const attempt = (name, fn) => {
    calls.length = 0;
    try { const v = fn(); out[name] = {value:v === undefined ? 'undefined' : v, calls:calls.slice()}; }
    catch(e) { out[name] = {error:e === marker ? 'marker' : e.name, message:e === marker ? '' : e.message, calls:calls.slice()}; }
  };
  attempt('eval', () => {let local=40;const input=script('local+2'),result=eval(input);return result===input&&typeof input==='object'?'identity':result});
  attempt('indirectEval', () => {const input=script('40+2'),result=(0,eval)(input);return result===input&&typeof input==='object'?'identity':result});
  attempt('evalNumber', () => eval(42));
  attempt('evalObject', () => {const o={toString(){throw marker}};return eval(o)===o});
  attempt('evalBoxed', () => {const o=new String('40+2');return eval(o)===o});
  attempt('Function', () => Function(script('return 42'))());
  attempt('FunctionArgs', () => Function(script('a'),script('return a+1'))(41));
  attempt('FunctionMixed', () => Function('a',script('return a+1'))(41));
  attempt('FunctionEmpty', () => typeof Function());
  attempt('FunctionConstructor', () => (()=>{}).constructor(script('return 42'))());
  attempt('AsyncFunction', () => typeof Object.getPrototypeOf(async function(){}).constructor(script('return 42')));
  attempt('GeneratorFunction', () => Object.getPrototypeOf(function*(){}).constructor(script('yield 42'))().next().value);
  attempt('AsyncGeneratorFunction', () => typeof Object.getPrototypeOf(async function*(){}).constructor(script('yield 42')));
  attempt('innerHTML', () => {box.innerHTML=html('<b>ok</b>');return box.innerHTML});
  attempt('innerHTMLNull', () => {box.innerHTML=null;return box.innerHTML});
  attempt('outerHTML', () => {const e=document.createElement('i');box.appendChild(e);e.outerHTML=html('<b>ok</b>')});
  attempt('outerHTMLDetached', () => {document.createElement('i').outerHTML=html('<b>ok</b>')});
  attempt('insertAdjacentHTML', () => box.insertAdjacentHTML('beforeend',html('<b>ok</b>')));
  attempt('adjacentBadPosition', () => box.insertAdjacentHTML('wrong',html('<b>ok</b>')));
  attempt('setHTMLUnsafe', () => box.setHTMLUnsafe(html('<b>ok</b>')));
  attempt('shadowInnerHTML', () => {const e=document.createElement('div'),r=e.attachShadow({mode:'open'});r.innerHTML=html('<b>ok</b>')});
  attempt('shadowSetHTMLUnsafe', () => {const e=document.createElement('div'),r=e.attachShadow({mode:'open'});r.setHTMLUnsafe(html('<b>ok</b>'))});
  attempt('DOMParserHTML', () => new DOMParser().parseFromString(html('<b>ok</b>'),'text/html').body.textContent);
  attempt('DOMParserXML', () => new DOMParser().parseFromString(html('<b>ok</b>'),'text/xml').documentElement.tagName);
  attempt('contextualFragment', () => document.createRange().createContextualFragment(html('<b>ok</b>')).textContent);
  attempt('parseHTMLUnsafe', () => Document.parseHTMLUnsafe(html('<b>ok</b>')).body.textContent);
  attempt('scriptSrc', () => {elem().src=url('/script.js')});
  attempt('scriptAttr', () => elem().setAttribute('SRC',url('/script.js')));
  attempt('scriptAttrNS', () => elem().setAttributeNS(null,'src',url('/script.js')));
  attempt('scriptAttrOtherNS', () => elem().setAttributeNS('urn:other','src',url('/script.js')));
  attempt('iframeSrcdoc', () => {document.createElement('iframe').srcdoc=html('<b>ok</b>')});
  attempt('iframeSrcdocAttr', () => document.createElement('iframe').setAttribute('srcdoc',html('<b>ok</b>')));
  attempt('scriptText', () => {elem().text=script('40+2')});
  attempt('scriptTextContent', () => {elem().textContent=script('40+2')});
  attempt('scriptInnerText', () => {elem().innerText=script('40+2')});
  attempt('scriptTextNode', () => elem().appendChild(document.createTextNode('40+2')).textContent);
  attempt('scriptAppend', () => elem().append('40+2'));
  attempt('scriptNodeTextContent', () => Object.getOwnPropertyDescriptor(Node.prototype,'textContent').set.call(elem(),script('40+2')));
  attempt('eventAttr', () => box.setAttribute('onclick',script('40+2')));
  attempt('unknownEventAttr', () => box.setAttribute('onmadeup',script('40+2')));
  attempt('eventProperty', () => {box.onclick=script('40+2');return box.onclick===null});
  attempt('svgScriptHref', () => document.createElementNS('http://www.w3.org/2000/svg','script').setAttribute('href',url('/script.js')));
  attempt('svgScriptXlink', () => document.createElementNS('http://www.w3.org/2000/svg','script').setAttributeNS('http://www.w3.org/1999/xlink','xlink:href',url('/script.js')));
  attempt('svgBaseVal', () => {document.createElementNS('http://www.w3.org/2000/svg','script').href.baseVal=url('/script.js')});
  attempt('objectCodeBase', () => {document.createElement('object').codeBase=url('/script.js')});
  attempt('customNamespaceEvent', () => document.createElementNS('urn:other','element').setAttribute('onclick',script('40+2')));
  attempt('mathEvent', () => document.createElementNS('http://www.w3.org/1998/Math/MathML','math').setAttribute('onclick',script('40+2')));
  attempt('objectData', () => {document.createElement('object').data=url('/script.js')});
  attempt('embedSrc', () => {document.createElement('embed').src=url('/script.js')});
  attempt('timer', () => {clearTimeout(setTimeout(script('40+2'),10000))});
  attempt('interval', () => {clearInterval(setInterval(script('40+2'),10000))});
  attempt('timerFunction', () => {clearTimeout(setTimeout(()=>{},10000))});
  attempt('Worker', () => {new Worker(url('/script.js')).terminate()});
  attempt('SharedWorker', () => {new SharedWorker(url('/script.js')).port.close()});
  attempt('attributeNode', () => {const a=document.createAttribute('src');a.value='/script.js';elem().setAttributeNode(a)});
  attempt('attachedAttributeValue', () => {const e=elem();e.setAttribute('src',p.createScriptURL(''));e.getAttributeNode('src').value=url('/script.js')});
  attempt('documentWrite', () => {document.write(html('<b>ok</b>'))});
  attempt('documentWritelnMixed', () => {document.writeln(p.createHTML('<i>'),html('ok</i>'))});
  return out;
})()
