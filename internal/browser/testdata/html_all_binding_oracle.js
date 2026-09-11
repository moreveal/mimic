(() => {
  const out={},f=document.createElement('iframe');document.body.appendChild(f);
  try {
    const c=f.contentWindow,d=f.contentDocument;
    d.body.innerHTML='<div id="unique"></div><input name="pair"><input name="pair">';
    const all=d.all,item=HTMLAllCollection.prototype.item,named=HTMLAllCollection.prototype.namedItem,length=Object.getOwnPropertyDescriptor(HTMLAllCollection.prototype,'length').get;
    const attempt=fn=>{try{return {ok:true,type:typeof fn()}}catch(e){return {ok:false,name:e.name,local:e instanceof TypeError}}};
    out.borrowed={item:item.call(all,'unique')===d.getElementById('unique'),named:named.call(all,'unique')===d.getElementById('unique'),length:length.call(all)===all.length,namedCollection:named.call(all,'pair')===all.namedItem('pair'),ownerPrototype:Object.getPrototypeOf(named.call(all,'pair'))===c.HTMLCollection.prototype,reverse:c.HTMLAllCollection.prototype.item.call(document.all,0)===document.documentElement};
    let conversions=0;const arg={toString(){conversions++;return 'unique'}};
    out.forged=attempt(()=>item.call(Object.create(HTMLAllCollection.prototype),arg));out.invalidConversions=conversions;
    out.symbol=attempt(()=>item.call(all,Symbol('x')));out.namedMissing=attempt(()=>named.call(all));out.itemMissing=item.call(all)===null;
    out.converted=item.call(all,arg)===d.getElementById('unique');out.conversions=conversions;
    out.conversionPaths={};
    for(const [operation,fn] of [['item',value=>item.call(all,value)],['call',value=>all(value)],['namedItem',value=>named.call(all,value)]]){
      const row={};
      for(const value of ['unique','0','missing']){
        const hints=[],argument={[Symbol.toPrimitive](hint){hints.push(hint);return value}};
        const result=fn(argument);row[value]={hints,result:result===null?null:result.tagName};
      }
      const hints=[],changing={[Symbol.toPrimitive](hint){hints.push(hint);return hints.length===1?'unique':'missing'}};
      row.changing={result:fn(changing)===null,hints};out.conversionPaths[operation]=row;
    }
    all.item=()=>{throw Error('public override')};all.namedItem=()=>{throw Error('public override')};
    out.overrideIgnored=item.call(all,'unique')===d.getElementById('unique')&&named.call(all,'pair').length===2;
    d.body.appendChild(d.createElement('span'));out.live=length.call(all)===all.length;
    f.remove();out.retained=item.call(all,'unique')===d.getElementById('unique');
    return out;
  }finally{f.remove()}
})()
