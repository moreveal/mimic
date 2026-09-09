(()=>{
 const out={};
 for(const property of ['flex','-webkit-flex','flex-flow','-webkit-flex-flow']){
  out[property]={};for(const value of ['none','auto','initial','inherit','1','0','2 3','1 0 auto','auto 2','2 auto 3','1 2 0','0 0 0','2px','20%','content','min-content','max-content','fit-content','stretch','calc(10px + 2%)','-1','1 -1','1 2 -3px','1 2 3 4','bogus','row','row nowrap','row wrap','wrap row-reverse','column wrap-reverse','wrap','nowrap','row column','wrap nowrap','ROW WRAP','1e2 2.50 10PX']){
   const s=document.createElement('div').style;s.setProperty(property,value);out[property][value]={text:s.cssText,value:s.getPropertyValue(property),supported:CSS.supports(property,value),components:Array.from({length:s.length},(_,i)=>[s.item(i),s.getPropertyValue(s.item(i))])};
  }
 }
 const s=document.createElement('div').style;s.webkitFlex='auto';s.flexGrow='3';out.mutation=[s.cssText,s.webkitFlex];s.webkitFlex='-1';out.invalidPreserves=s.cssText;
 return out;
})()
