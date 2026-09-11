(() => {
  const attempt=fn=>{try{return fn();}catch(error){return {error:error.name};}};
  const nested=globalThis.__retainedNestedWindow;
  return {
    title:attempt(()=>nested.document.title),
    self:attempt(()=>nested.window===nested),
    defaultViewNull:attempt(()=>nested.document.defaultView===null),
    eval:attempt(()=>nested.eval('17')),
    bodyReadable:attempt(()=>nested.document.body!==null),
  };
})()
