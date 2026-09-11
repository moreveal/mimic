(async () => {
  const frame = document.createElement('iframe');
  document.body.appendChild(frame);
  const oldWindow = frame.contentWindow, oldDocument = frame.contentDocument;
  const events = [];
  const report = value => events.push(value);
  const schedule = oldWindow.eval(`((report)=>{
    queueMicrotask(()=>report('microtask'));
    Promise.resolve().then(()=>report('promise'));
    setTimeout(()=>report('timer'), 0);
    return document;
  })`);
  schedule(report);
  await new Promise(resolve => setTimeout(resolve, 100));
  const activeEvents = events.slice();
  events.length = 0;
  await new Promise(resolve => { frame.onload = resolve; frame.src = '/navigation-realm-lifecycle'; });
  const sameDocument = schedule(report) === oldDocument;
  await new Promise(resolve => setTimeout(resolve, 100));
  const afterNavigation = {sameDocument, events:events.slice()};
  const currentDocument = frame.contentDocument;
  const currentWindow = frame.contentWindow;
  const currentRead = currentWindow.eval('(()=>document)');
  const currentEval = currentWindow.eval;
  frame.remove();
  return {activeEvents, afterNavigation, detached:{contentWindowNull:frame.contentWindow===null,
    oldDocumentReadable:oldDocument.body!==null,
    currentDocumentReadable:currentDocument.body!==null,
    currentFunctionDocument:currentRead()===currentDocument,
    savedEvalPrimitive:currentEval('17'),
    savedEvalDocument:currentEval('document')===currentDocument,
    oldDefaultViewNull:oldDocument.defaultView===null,
    detachedDefaultViewNull:currentDocument.defaultView===null}};
})()
