(async () => {
  const attempt = fn => { try { const value=fn(); return value===undefined?'undefined':value; } catch(error) { return {error:error.name}; } };
  const frame = document.createElement('iframe');
  document.body.appendChild(frame);
  // Export only the WindowProxy. Do not export a Document, function or ordinary
  // object from the nested realm before its ancestor navigates.
  const nested = frame.contentWindow.eval(`(()=>{
    const inner=document.createElement('iframe');document.body.appendChild(inner);
    inner.contentDocument.title='retained nested';
    return inner.contentWindow;
  })()`);
  await new Promise(resolve=>{frame.onload=resolve;frame.src='/navigation-window-edges';});
  const nestedResult = {
    title:attempt(()=>nested.document.title),
    self:attempt(()=>nested.window===nested),
    defaultViewNull:attempt(()=>nested.document.defaultView===null),
    eval:attempt(()=>nested.eval('17')),
    bodyReadable:attempt(()=>nested.document.body!==null),
  };
  frame.remove();
  const evalFrame=document.createElement('iframe');document.body.appendChild(evalFrame);
  const oldEval=evalFrame.contentWindow.eval;
  const target=new URL('/navigation-eval-edges',location.href);target.hostname='localhost';
  await new Promise(resolve=>{evalFrame.onload=resolve;evalFrame.src=target.href;});
  const evalResult={
    direct:attempt(()=>oldEval('17')),
    call:attempt(()=>oldEval.call(null,'17')),
    apply:attempt(()=>oldEval.apply(null,['17'])),
    bound:attempt(()=>oldEval.bind(null)('17')),
    reflectApply:attempt(()=>Reflect.apply(oldEval,null,['17'])),
  };
  evalFrame.remove();
  return {nested:nestedResult, eval:evalResult};
})()
