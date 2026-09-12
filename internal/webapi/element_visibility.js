// Observe the existing box/style model; zero-area boxes still count as boxes.
Object.defineProperty(Element.prototype,'checkVisibility',{
  value:function(options={}){
    if(!elementSlot(this))throw new TypeError('Illegal invocation');
    if(!this.isConnected)return false;
    options=options??{};
    const own=getComputedStyle(this);
    if(own.display==='none'||own.display==='contents')return false;
    if((options.visibilityProperty||options.checkVisibilityCSS)&&['hidden','collapse'].includes(own.visibility))return false;
    for(let element=this;element;element=geometryParent(element)){
      const style=getComputedStyle(element);
      if(style.display==='none')return false;
      if(element!==this&&style.contentVisibility==='hidden')return false;
      if((options.opacityProperty||options.checkOpacity)&&style.opacity==='0')return false;
    }
    return true;
  },writable:true,enumerable:true,configurable:true
});
