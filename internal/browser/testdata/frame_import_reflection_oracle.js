(() => {
  const frame = document.createElement('iframe');
  document.body.appendChild(frame);
  const child = frame.contentWindow;
  const foreign = child.eval('({items:[1,2],fn:function(){return 42}})');
  const original = Object.getOwnPropertyDescriptor;
  const describe = child.Object.getOwnPropertyDescriptor;
  let privateReads = 0, answer;
  Object.getOwnPropertyDescriptor = function(object, key) {
    if (++privateReads > 12) throw Error('recursive private frame reflection');
    return describe(object, key);
  };
  try {
    answer = foreign.items[0] + foreign.fn();
  } finally {
    Object.getOwnPropertyDescriptor = original;
    frame.remove();
  }
  return JSON.stringify({answer, privateReads});
})()
