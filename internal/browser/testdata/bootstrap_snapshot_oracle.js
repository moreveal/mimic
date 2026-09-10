(async()=>{
 globalThis.snapshotTokenObject={};
 const first=document.createElement('iframe');document.body.appendChild(first);
 const populate=label=>{
  document.title=label;
  const form=document.createElement('form');form.id='form';
  const input=document.createElement('input');input.name='field';input.id='field';input.value=label;form.appendChild(input);document.body.appendChild(form);
  const host=document.createElement('div');host.id='host';document.body.appendChild(host);
  const shadow=host.attachShadow({mode:'open'}),span=document.createElement('span');span.id='shadow-child';span.textContent=label;shadow.appendChild(span);
  const other=document.implementation.createHTMLDocument(label+' secondary'),node=other.createElement('p');node.id='secondary';other.body.appendChild(node);
  globalThis.snapshotState={form,input,host,shadow,span,other,node,label};
  globalThis.snapshotKey=Symbol('same');globalThis.snapshotObject={[snapshotKey]:label};
  parent.snapshotTokenObject[Symbol('token')]=label;
  return true;
 };
 const check=()=>{
  const s=snapshotState;
  return {
   canonical:document===window.document&&document.defaultView===window&&document.documentElement.ownerDocument===document&&document.body.ownerDocument===document,
   constructors:document instanceof Document&&s.input instanceof HTMLInputElement&&Object.getPrototypeOf(s.input)===HTMLInputElement.prototype,
   selector:document.querySelector('#field')===s.input&&document.querySelectorAll('input')[0]===s.input&&document.getElementById('field')===s.input,
   form:s.input.form===s.form&&s.form.elements.namedItem('field')===s.input&&s.input.value===s.label,
   shadow:s.host.shadowRoot===s.shadow&&s.shadow.host===s.host&&s.shadow.querySelector('#shadow-child')===s.span&&s.span.getRootNode()===s.shadow&&s.span.getRootNode({composed:true})===document&&document.querySelector('#shadow-child')===null,
   secondary:s.node.ownerDocument===s.other&&s.other.body.ownerDocument===s.other&&s.other.querySelector('#secondary')===s.node&&document.querySelector('#secondary')===null,
   title:document.title===s.label,
   intl:new Intl.NumberFormat('en-US',{useGrouping:false}).format(1234.5)==='1234.5'
  };
 };
 const a=first.contentWindow;a.eval('('+populate.toString()+')("first")');
 const second=document.createElement('iframe');document.body.appendChild(second);const b=second.contentWindow;
 const pristine=b.eval("typeof snapshotState==='undefined'&&document.title===''&&document.querySelector('input')===null");
 b.eval('('+populate.toString()+')("second")');
 const firstChecks=a.eval('('+check.toString()+')()'),secondChecks=b.eval('('+check.toString()+')()');
 const keyA=Reflect.ownKeys(a.snapshotObject)[0],keyB=Reflect.ownKeys(b.snapshotObject)[0];
 // Foreign Symbols used against another foreign target are a preexisting
 // bridge limitation. This tests imported identity and local token isolation.
 const tokenKeys=Reflect.ownKeys(snapshotTokenObject);
 const symbols=keyA!==keyB&&a.snapshotObject[keyA]==='first'&&b.snapshotObject[keyB]==='second'&&tokenKeys.length===2&&snapshotTokenObject[tokenKeys[0]]==='first'&&snapshotTokenObject[tokenKeys[1]]==='second';
 const realmIdentity=a.Document!==b.Document&&a.Document!==Document&&a.Array!==b.Array&&Object.getPrototypeOf(a.snapshotState.input)===a.HTMLInputElement.prototype;
 const parentState=document.querySelector('#field')===null&&a.parent===window&&b.parent===window&&a.top===window&&b.top===window;
 const retained=a.snapshotState.input;first.remove();const detached=b.snapshotState.input.value==='second'&&retained.value==='first';
 second.remove();
 return {pristine,first:firstChecks,second:secondChecks,symbols,realmIdentity,parentState,detached};
})()
