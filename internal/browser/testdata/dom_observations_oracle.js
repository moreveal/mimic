(()=>{
 const d=document.implementation.createHTMLDocument();d.body.innerHTML='<div id="a" x="1" y="2"><span id="b"></span></div><p id="c"></p><a name="old"></a><a href="/a"></a><area href="/b"><form id="form"></form><img id="pic"><embed id="plug"><applet></applet>';
 const a=d.getElementById('a'),b=d.getElementById('b'),c=d.getElementById('c'),x=a.getAttributeNode('x'),y=a.getAttributeNode('y');
 const r={position:[a.compareDocumentPosition(a),a.compareDocumentPosition(b),b.compareDocumentPosition(a),a.compareDocumentPosition(c),c.compareDocumentPosition(a),a.compareDocumentPosition(x),x.compareDocumentPosition(a),x.compareDocumentPosition(y),y.compareDocumentPosition(x),x.compareDocumentPosition(b),b.compareDocumentPosition(x)],collections:{}};
 for(const name of ['children','links','anchors','forms','images','embeds','plugins','applets','scripts']){const v=d[name];r.collections[name]={type:Object.prototype.toString.call(v),length:v.length,same:v===d[name]}}
 r.pluginsSame=d.plugins===d.embeds;r.childElements=[d.childElementCount,d.firstElementChild===d.documentElement,d.lastElementChild===d.documentElement];
 const images=d.images;d.body.appendChild(d.createElement('img'));r.live=images.length;
 const parser=new DOMParser();r.parsed=[];
 for(const [text,type]of [['<title> x </title><p id="p">hello</p><script>globalThis.parserExecuted=1</script>','text/html'],['<!doctype html><p>ok','text/html'],['<root xmlns="urn:test"><child a="b"/></root>','application/xml'],['<root>','text/xml'],['<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>','image/svg+xml']]){const p=parser.parseFromString(text,type);r.parsed.push({type:p.contentType,node:p.documentElement.localName,ns:p.documentElement.namespaceURI,body:p.body?.textContent||null,url:p.URL,location:p.location,view:p.defaultView,ready:p.readyState,compat:p.compatMode,owner:p.documentElement.ownerDocument===p,connected:p.documentElement.isConnected,html:p.contentType==='text/html'?p.body.innerHTML:null})}
 r.inert=globalThis.parserExecuted===undefined;try{parser.parseFromString('', 'TEXT/HTML')}catch(e){r.invalidType=e.name}try{DOMParser.prototype.parseFromString.call({},'', 'text/html')}catch(e){r.invalidReceiver=e.name}
 return r;
})()
