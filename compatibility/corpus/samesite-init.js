(()=>{
  for(const [name,attribute] of [['none','; SameSite=None'],['lax','; SameSite=Lax'],['strict','; SameSite=Strict'],['default','']])document.cookie='nav_'+name+'=1; Secure; Path=/'+attribute;
  return document.cookie;
})()
