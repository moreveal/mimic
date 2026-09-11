(() => {
  const out={},root=document.createElement('div');root.innerHTML='<b id="first"></b><b id="second"></b>';
  const frame=document.createElement('iframe');document.body.appendChild(frame);
  const cap=fn=>{try{return {value:fn()}}catch(e){return {error:e.name,local:e instanceof TypeError}}};
  const id=node=>node===null?null:node?.id;
  const fragment=document.createDocumentFragment();fragment.appendChild(root.cloneNode(true));
  const form=document.createElement('form');form.innerHTML='<input id="control" name="entry"><select><option id="option">one</option></select>';document.body.appendChild(form);
  try {
    const select=form.querySelector('select');
    for(const [name,list,prototype] of [
      ['live',root.childNodes,NodeList.prototype],['static',root.querySelectorAll('b'),NodeList.prototype],
      ['fragment',fragment.querySelectorAll('b'),NodeList.prototype],['children',root.children,HTMLCollection.prototype],
      ['form',form.elements,HTMLCollection.prototype],['options',select.options,HTMLCollection.prototype],['selected',select.selectedOptions,HTMLCollection.prototype]
    ]){
      const item=prototype.item,get=Object.getOwnPropertyDescriptor(prototype,'length').get;
      const row={stable:item===prototype.item,prototypeMethod:list.item===item,missing:cap(()=>item.call(list)),
        empty:cap(()=>id(item.call(list,100))),wrapped:cap(()=>id(item.call(list,4294967296))),
        negative:cap(()=>id(item.call(list,-1))),bigint:cap(()=>item.call(list,0n)),
        symbol:cap(()=>item.call(list,Symbol())),length:cap(()=>get.call(list)),
        forged:cap(()=>get.call(Object.create(prototype))),proxy:cap(()=>get.call(new Proxy(list,{})))};
      let converted=0;row.invalid=cap(()=>item.call({}, {valueOf(){converted++;return 0}}));row.invalidConversions=converted;
      const saved=Object.getPrototypeOf(list);Object.setPrototypeOf(list,null);
      try{row.noPrototype=cap(()=>id(item.call(list,0)))}finally{Object.setPrototypeOf(list,saved)}
      if(prototype===HTMLCollection.prototype){
        const named=prototype.namedItem;row.named=cap(()=>id(named.call(list,item.call(list,0)?.id)));
        row.namedMissing=cap(()=>named.call(list));row.namedSymbol=cap(()=>named.call(list,Symbol()));
      }
      out[name]=row;
    }
    out.aliases={values:NodeList.prototype.values===Array.prototype.values,entries:NodeList.prototype.entries===Array.prototype.entries,keys:NodeList.prototype.keys===Array.prototype.keys,forEach:NodeList.prototype.forEach===Array.prototype.forEach,iterator:NodeList.prototype[Symbol.iterator]===Array.prototype.values,htmlIterator:HTMLCollection.prototype[Symbol.iterator]===Array.prototype.values};
    out.generic={values:cap(()=>Array.from(NodeList.prototype.values.call({0:'a',1:'b',length:2}))),item:cap(()=>NodeList.prototype.item.call({0:'a',length:1},0))};
    for(const key of ['item','values','forEach']){const list=root.childNodes;Object.defineProperty(list,key,{value:'shadow',configurable:true});out['shadow.'+key]=list[key];delete list[key]}
    const list=root.childNodes,iterator=list.values();out.liveFirst=id(iterator.next().value);
    root.lastChild.remove();const added=document.createElement('i');added.id='added';root.appendChild(added);
    out.liveRemainder=Array.from(iterator,id);
    const foreign=frame.contentWindow;foreign.document.body.innerHTML='<p id="child"></p>';
    out.borrowed={node:NodeList.prototype.item.call(foreign.document.body.childNodes,0)===foreign.document.body.firstChild,html:HTMLCollection.prototype.item.call(foreign.document.body.children,0)===foreign.document.body.firstChild,reverse:foreign.NodeList.prototype.item.call(root.childNodes,0)===root.firstChild};
    out.iteratorShape={name:NodeList.prototype.values.name,length:NodeList.prototype.values.length,source:Function.prototype.toString.call(NodeList.prototype.values),tag:Object.prototype.toString.call(root.childNodes.values())};
    return out;
  }finally{form.remove();frame.remove()}
})()
