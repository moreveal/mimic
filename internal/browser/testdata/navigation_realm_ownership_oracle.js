(async () => {
  const results = {};
  const attempt = fn => { try { return fn(); } catch (error) { return {error: error.name}; } };
  for (const crossOrigin of [false, true]) {
    const frame = document.createElement('iframe');
    document.body.appendChild(frame);
    const windowProxy = frame.contentWindow;
    const oldDocument = windowProxy.document;
    const oldEval = windowProxy.eval;
    const oldIdentity = windowProxy.eval('((value)=>value)');
    const oldSetSymbol = windowProxy.eval('((object,key,value)=>{object[key]=value;return Reflect.ownKeys(object).includes(key)})');
    const oldObject = windowProxy.eval('({value:17})');
    const oldObjectConstructor = windowProxy.Object;
    const oldDocumentConstructor = windowProxy.HTMLDocument;
    const oldSymbol = windowProxy.eval('globalThis.savedSymbol=Symbol("retained")');
    const readSymbol = windowProxy.eval('(()=>savedSymbol)');
    const readDocument = windowProxy.eval('(()=>document)');
    const readWindow = windowProxy.eval('(()=>window)');
    const readGlobal = windowProxy.eval('(()=>globalThis.savedMarker)');
    windowProxy.savedMarker = 41;
    oldDocument.savedValue = 23;
    const oldBody = oldDocument.body;
    const target = new URL('/navigation-realm-next', location.href);
    if (crossOrigin) target.hostname = target.hostname === 'localhost' ? '127.0.0.1' : 'localhost';
    await new Promise(resolve => { frame.onload = resolve; frame.src = target.href; });
    results[crossOrigin ? 'crossOrigin' : 'sameOrigin'] = {
      windowStable: windowProxy === frame.contentWindow,
      currentDocument: attempt(() => windowProxy.document === oldDocument),
      contentDocumentNull: frame.contentDocument === null,
      oldDocumentValue: attempt(() => oldDocument.savedValue),
      oldDocumentBody: attempt(() => oldDocument.body === oldBody),
      oldDocumentConstructor: attempt(() => oldDocument.constructor === oldDocumentConstructor),
      oldDefaultViewNull: attempt(() => oldDocument.defaultView === null),
      oldObjectValue: attempt(() => oldObject.value),
      oldObjectPrototype: attempt(() => Object.getPrototypeOf(oldObject) === oldObjectConstructor.prototype),
      constructOldObject: attempt(() => Object.getPrototypeOf(new oldObjectConstructor()) === oldObjectConstructor.prototype),
      oldSymbolIdentity: attempt(() => readSymbol() === oldSymbol),
      oldFunctionDocument: attempt(() => readDocument() === oldDocument),
      oldFunctionWindow: attempt(() => readWindow() === windowProxy),
      oldEvalDocument: attempt(() => oldEval('document') === oldDocument),
      oldEvalWindow: attempt(() => oldEval('window') === windowProxy),
      oldEvalResultType: attempt(() => typeof oldEval('document')),
      oldEvalPrimitive: attempt(() => { const value = oldEval('17'); return value === undefined ? 'undefined' : value; }),
      oldEvalObjectArgument: attempt(() => { const value = oldEval(oldObject); return value === undefined ? 'undefined' : value === oldObject; }),
      oldEvalThrow: attempt(() => { const value = oldEval('throw new Error("eval executed")'); return value === undefined ? 'undefined' : value; }),
      oldWindowRoundtrip: attempt(() => oldIdentity(windowProxy) === windowProxy),
      parentWindowRoundtrip: attempt(() => oldIdentity(window) === window),
      oldSymbolRoundtrip: attempt(() => oldIdentity(oldSymbol) === oldSymbol),
      oldSymbolKey: attempt(() => oldSetSymbol(oldObject, oldSymbol, 53) && oldObject[oldSymbol] === 53 && Reflect.ownKeys(oldObject).includes(oldSymbol)),
      newWindowRoundtrip: attempt(() => windowProxy.eval('((value)=>value)')(oldIdentity(windowProxy)) === windowProxy),
      newSymbolKey: attempt(() => windowProxy.eval('((object,key)=>Reflect.ownKeys(object).includes(key)&&object[key]===53)')(oldObject, oldSymbol)),
      oldFunctionGlobal: attempt(() => readGlobal() === undefined),
      oldDocumentMutation: attempt(() => {
        const element = oldDocument.createElement('p');
        element.id = 'retained-old';
        oldBody.appendChild(element);
        return oldDocument.getElementById('retained-old') === element && element.ownerDocument === oldDocument;
      }),
      currentHasOldMutation: attempt(() => windowProxy.document.getElementById('retained-old') !== null),
      currentHasOldGlobal: attempt(() => 'savedMarker' in windowProxy),
      currentConstructorReplaced: attempt(() => windowProxy.Object !== oldObjectConstructor),
    };
    frame.remove();
  }
  return results;
})()
