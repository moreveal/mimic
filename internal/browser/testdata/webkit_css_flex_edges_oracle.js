(()=>{
 const out={};for(const property of ['flex','flex-grow','flex-shrink','flex-basis','flex-direction','flex-wrap']){
  out[property]={};for(const value of ['INITIAL','INHERIT','UNSET','0','-0','+2.5','1e-2','-0px','1.','NaN','Infinity','1e999','-2px','0em','2rem','calc(2px + 3px)','calc(10px - 20px)','calc(10px+2%)','calc(2% - 10px)','bogus']){
   const s=document.createElement('div').style;s.setProperty(property,value);out[property][value]=[s.cssText,s.getPropertyValue(property),CSS.supports(property,value)];
  }
 }return out;
})()
