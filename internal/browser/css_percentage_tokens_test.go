package browser

import "testing"

func TestCSSPercentageAdjacentTokens(t *testing.T) {
	parallelBrowserTest(t)
	p := bootstrapSnapshotPage(t)
	historyEval(t, p, `(()=>{
 const sheet=new CSSStyleSheet();sheet.replaceSync('.center{inset:50% auto auto 50%;margin:10%auto;padding:10%20%}');
 const style=sheet.cssRules[0].style,inline=document.createElement('div').style;
 inline.cssText='inset:50%auto auto 50%;margin:10%auto;padding:10%20%';
 for(const s of [style,inline])if(s.getPropertyValue('inset')!=='50% auto auto 50%'||s.getPropertyValue('margin')!=='10% auto'||s.getPropertyValue('padding')!=='10% 20%')return s.cssText;
 return true;
 })()`, true)
}
