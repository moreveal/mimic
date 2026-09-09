(()=>{
 const out={};for(const value of ['initial','inherit','var(--choice)']){
  const s=document.createElement('div').style;s.webkitFlex=value;s.flexGrow='2';
  out[value]={text:s.cssText,value:s.webkitFlex,components:Array.from({length:s.length},(_,i)=>[s.item(i),s.getPropertyValue(s.item(i))])};
 }
 const s=document.createElement('div').style;s.setProperty('-webkit-flex','initial','important');out.priority=[s.getPropertyPriority('-webkit-flex'),s.getPropertyPriority('flex-grow')];s.setProperty('flex-grow','initial');out.mixed=[s.cssText,s.webkitFlex,s.getPropertyPriority('flex')];return out;
})()
