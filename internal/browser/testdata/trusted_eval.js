(() => {
  const policy = trustedTypes.createPolicy('eval-regression', {
    createScript: value => value,
    createHTML: value => value,
  });
  const code = value => policy.createScript(value);
  let local = 7;
  const direct = eval(code('local + 1'));
  const ordinary = eval('local + 2');
  eval(code('function declaredHere() { return 19; }'));
  (0, eval)(code('function declaredGlobally() { return 23; }'));
  const object = { toString() { throw Error('must not coerce'); } };
  const boxed = new String('40 + 2');
  const html = policy.createHTML('40 + 2');
  const forged = Object.create(TrustedScript.prototype);
  const proxy = new Proxy(code('40 + 2'), {});
  const tampered = code('21 * 2');
  tampered.toString = () => '99';
  Object.setPrototypeOf(tampered, null);
  const brand = trustedTypes.isScript(tampered);
  const tamperedResult = eval(tampered);
  const intrinsicCode = code('6 * 7'), get = WeakMap.prototype.get;
  let intrinsicResult;
  try {
    WeakMap.prototype.get = () => { throw Error('must use internal state'); };
    intrinsicResult = eval(intrinsicCode);
  } finally { WeakMap.prototype.get = get; }
  const thrown = new RangeError('identity');
  let sameException = false, syntaxError = false;
  try { eval(code('throw thrown')); } catch (error) { sameException = error === thrown; }
  try { eval(code('function {')); } catch (error) { syntaxError = error instanceof SyntaxError; }
  return {
    direct, ordinary, localFunction: declaredHere(),
    localStayedLocal: !Object.hasOwn(globalThis, 'declaredHere'),
    globalFunction: globalThis.declaredGlobally(),
    object: eval(object) === object, boxed: eval(boxed) === boxed,
    html: eval(html) === html, forged: eval(forged) === forged,
    proxy: eval(proxy) === proxy, number: eval(17) === 17,
    empty: eval(trustedTypes.emptyScript) === undefined,
    brand, tamperedResult, intrinsicResult, sameException, syntaxError,
    resolverHidden: !Object.hasOwn(globalThis, '__mimicEvalSourceResolver'),
  };
})()
