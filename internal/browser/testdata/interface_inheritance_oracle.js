(() => {
  const pairs = [['Element','Node'],['HTMLElement','Element'],['HTMLDivElement','HTMLElement'],['Document','Node'],['HTMLDocument','Document'],['XMLDocument','Document'],['MouseEvent','UIEvent'],['UIEvent','Event'],['DOMException','Error'],['AbsoluteOrientationSensor','OrientationSensor'],['OrientationSensor','Sensor'],['AnalyserNode','AudioNode']];
  const inspect = w => Object.fromEntries(pairs.map(([child,parent]) => [child, {
    constructorParent:Object.getPrototypeOf(w[child])===w[parent],
    instanceParent:Object.getPrototypeOf(w[child].prototype)===w[parent].prototype,
    ownConstructor:w[child].prototype.constructor===w[child],
    functionPrototype:w.Function.prototype.isPrototypeOf(w[child]),
  }]));
  const frame=document.createElement('iframe');document.body.appendChild(frame);
  const xhr = () => {const x=new XMLHttpRequest();let illegal=false;try{new XMLHttpRequestEventTarget()}catch(e){illegal=e instanceof TypeError}return {parent:Object.getPrototypeOf(XMLHttpRequest)===XMLHttpRequestEventTarget,brand:x instanceof XMLHttpRequestEventTarget,state:x.readyState,illegal};};
  try{return {main:inspect(window),child:inspect(frame.contentWindow),separate:frame.contentWindow.Element!==Element,xhr:xhr()};}
  finally{frame.remove();}
})()
