(()=>{
 const f=document.createElement('iframe');document.body.appendChild(f);const w=f.contentWindow,d=w.document,out={};
 const run=(name,fn)=>{try{out[name]=fn()}catch(e){out[name]={error:e.name,message:e.message}}};
 const local=document.createElement('div');local.id='cross-local';local.style.color='rgb(1, 2, 3)';const text=document.createTextNode('before');local.appendChild(text);document.body.appendChild(local);
 const foreign=d.createElement('section');foreign.id='cross-foreign';d.body.appendChild(foreign);
 run('foreignBrand',()=>({local:foreign instanceof Node,remote:foreign instanceof w.Node}));
 run('styleBorrow',()=>w.getComputedStyle(local).color);
 run('styleOwner',()=>{const style=document.createElement('style');style.textContent='#cross-local{padding-left:17px}';document.head.appendChild(style);const value=w.getComputedStyle(local).paddingLeft;style.remove();return value});
 run('append',()=>({same:d.body.appendChild(local)===local,owner:local.ownerDocument===d,parent:local.parentNode===d.body,connected:local.isConnected,child:text.ownerDocument===d,lookup:d.getElementById(local.id)===local,old:document.getElementById(local.id)===null}));
 run('insertBefore',()=>({same:document.body.insertBefore(foreign,f)===foreign,owner:foreign.ownerDocument===document,parent:foreign.parentNode===document.body,lookup:document.getElementById(foreign.id)===foreign}));
 run('remove',()=>({same:document.body.removeChild(foreign)===foreign,owner:foreign.ownerDocument===document,parent:foreign.parentNode===null,connected:foreign.isConnected}));
 run('adopt',()=>({same:d.adoptNode(foreign)===foreign,owner:foreign.ownerDocument===d}));
 run('import',()=>{const n=document.importNode(local,true);return {distinct:n!==local,owner:n.ownerDocument===document,text:n.textContent,realm:n instanceof Node}});
 run('position',()=>({contains:d.body.contains(local),mask:d.body.compareDocumentPosition(local)}));
 run('fragment',()=>{const fragment=d.createDocumentFragment(),a=d.createElement('a'),b=d.createTextNode('text');fragment.appendChild(a);fragment.appendChild(b);document.body.appendChild(fragment);return {empty:fragment.childNodes.length===0,owner:a.ownerDocument===document,parent:a.parentNode===document.body,text:b.parentNode===document.body}});
 run('borrowedMethods',()=>{const n=d.createElement('em');const same=w.Node.prototype.appendChild.call(document.body,n)===n;const contains=w.Node.prototype.contains.call(document.body,n);const removed=w.Node.prototype.removeChild.call(document.body,n)===n;return {same,contains,removed,owner:n.ownerDocument===document}});
 run('replaceAndVariadic',()=>{const box=d.createElement('div'),a=document.createElement('i'),b=document.createElement('b'),c=document.createElement('u');box.append(a,'x');const replaced=box.replaceChild(b,a)===a;box.prepend(c);const tree=Array.from(box.childNodes).map(n=>n.nodeName);b.replaceWith(a,'y');const after=Array.from(box.childNodes).map(n=>n.nodeName);return {replaced,tree,after,adopted:a.ownerDocument===d&&c.ownerDocument===d}});
 run('traversal',()=>{const n=document.createElement('div'),a=document.createElement('span');n.appendChild(a);const walker=d.createTreeWalker(n,NodeFilter.SHOW_ELEMENT);return {root:walker.root===n,next:walker.nextNode()===a}});
 run('spoof',()=>{try{document.body.appendChild(Object.create(w.Node.prototype));return 'accepted'}catch(e){return e.name}});
 local.remove();f.remove();return out;
})()
