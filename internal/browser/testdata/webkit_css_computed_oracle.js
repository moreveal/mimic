(()=>{
 const parent=document.createElement('div'),child=document.createElement('div');parent.append(child);document.body.append(parent);
 const names=['appearance','-webkit-appearance','-webkit-user-select','-webkit-user-drag','-webkit-user-modify','-webkit-box-align','-webkit-box-pack','-webkit-box-orient','-webkit-box-direction','-webkit-font-smoothing','-webkit-text-security','-webkit-print-color-adjust','-webkit-backface-visibility','-webkit-transform-style'];
 const alternate={'-webkit-user-modify':'read-write','-webkit-box-align':'center','-webkit-box-pack':'end','-webkit-box-orient':'vertical','-webkit-box-direction':'reverse','-webkit-text-security':'disc','-webkit-print-color-adjust':'exact','-webkit-backface-visibility':'hidden','-webkit-transform-style':'preserve-3d'};
 const result={};for(const name of names){parent.style.cssText='';child.style.cssText='';const read=()=>getComputedStyle(child).getPropertyValue(name);const row=[read()];parent.style.setProperty(name,alternate[name]||(name.includes('appearance')?'button':'none'));row.push(read());child.style.setProperty(name,'initial');row.push(read());child.style.setProperty(name,'inherit');row.push(read());result[name]=row;}
 parent.remove();return result;
})()
