(()=>{const out={},cases={
 '-webkit-animation-duration':['auto','1s','1000ms','-1s','1s, 2s'],
 '-webkit-animation-delay':['-1s','0ms','1s, -2s'],
 '-webkit-animation-timing-function':['ease','steps(2, jump-start)','steps(1, jump-none)','cubic-bezier(2, 0, 0, 1)'],
 '-webkit-animation-iteration-count':['infinite','0','2.5','-1'],
 '-webkit-animation-direction':['alternate','reverse, normal','bogus'],
 '-webkit-animation-fill-mode':['both','none, forwards','bogus'],
 '-webkit-animation-play-state':['paused','running, paused','bogus'],
 '-webkit-animation-name':['Spin','NONE','"Slide"','default','-','--x','spin, fade'],
 '-webkit-transition-property':['all','opacity, transform','none, opacity','default','-'],
 '-webkit-transition-duration':['auto','1s','-1s','1s, 2s'],
 '-webkit-transition-delay':['-1s','0ms'],
 '-webkit-transition-timing-function':['ease-in','steps(3)','steps(0)','cubic-bezier(0, 1, 1, 0)']};
 for(const [property,values] of Object.entries(cases)){out[property]={};for(const value of values){const s=document.createElement('div').style;s.setProperty(property,value);out[property][value]=[s.cssText,s.getPropertyValue(property),CSS.supports(property,value)];}}
 const s=document.createElement('div').style;s.webkitAnimation='Spin 1s';s.webkitAnimationDelay='2s';out.animation=[s.cssText,s.webkitAnimation];s.webkitAnimationDuration='-1s';out.invalid=s.cssText;
 s.cssText='';s.webkitTransition='opacity 1s';s.webkitTransitionDuration='2s';out.transition=[s.cssText,s.webkitTransition];return out;
})()
