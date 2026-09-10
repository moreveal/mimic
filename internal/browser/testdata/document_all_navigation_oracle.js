(async () => {
  const f=document.createElement('iframe');
  document.body.appendChild(f);
  try {
    const oldDocument=f.contentDocument, oldAll=oldDocument.all;
    const oldPrototype=Object.getPrototypeOf(oldAll), oldLength=oldAll.length;
    const loaded=new Promise(resolve=>f.onload=resolve);
    f.src='/document-all-navigation';
    await loaded;
    const current=f.contentDocument.all;
    const p=oldDocument.createElement('p');p.id='retained';oldDocument.body.appendChild(p);
    return {newDocument:f.contentDocument!==oldDocument,newCollection:current!==oldAll,oldIdentity:oldDocument.all===oldAll,oldPrototype:Object.getPrototypeOf(oldAll)===oldPrototype,newPrototype:Object.getPrototypeOf(current)===f.contentWindow.HTMLAllCollection.prototype,differentPrototypes:Object.getPrototypeOf(current)!==oldPrototype,oldLive:oldAll.length===oldLength+1,oldName:oldAll.retained===p,newName:current.newDocument===f.contentDocument.getElementById('newDocument'),oldType:typeof oldAll,oldBoolean:!!oldAll,oldEqualNull:oldAll==null,oldStrictUndefined:oldAll===undefined};
  } finally {f.remove();}
})()
