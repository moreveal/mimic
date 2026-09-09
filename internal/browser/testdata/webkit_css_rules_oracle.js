(()=>{
 const sheet=new CSSStyleSheet();sheet.replaceSync('div { -webkit-appearance: button; appearance: bogus; -webkit-user-select: none !important; }');
 const rule=sheet.cssRules[0],s=rule.style,out={initial:[rule.cssText,s.cssText,s.webkitAppearance,s.getPropertyValue('-webkit-appearance'),s.item(0),s.length]};
 s.webkitAppearance='none';out.write=[rule.cssText,s.appearance];s.setProperty('-webkit-appearance','invalid');out.invalid=s.cssText;
 out.remove=[s.removeProperty('-webkit-user-select'),s.cssText];
 return out;
})()
