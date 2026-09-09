(()=>{
 const e=document.createElement('div');document.body.append(e);const s=e.style,out={};
 for(const key of ['webkitAppearance','WebkitAppearance','-webkit-appearance','appearance','webkitNotReal'])out[key]={has:key in s,type:typeof s[key],own:Object.hasOwn(s,key)};
 s.webkitAppearance='button';out.write=[s.cssText,s.appearance,s.webkitAppearance,s.getPropertyValue('-webkit-appearance'),s.length,s.item(0),getComputedStyle(e).webkitAppearance];
 s.setProperty('-webkit-appearance','bogus');out.invalid=s.cssText;
 s.setProperty('webkitAppearance','none');out.camelMethod=s.cssText;
 s.setProperty('-WEBKIT-APPEARANCE','none','important');out.upper=[s.cssText,s.getPropertyPriority('appearance'),s.getPropertyPriority('-webkit-appearance')];
 out.remove=[s.removeProperty('-webkit-appearance'),s.cssText];
 s.cssText='appearance: button !important; -webkit-appearance: none';out.priority=[s.cssText,s.webkitAppearance];
 s.cssText='-webkit-appearance: auto';out.reset=[s.cssText,s.appearance];
 e.remove();return out;
})()
