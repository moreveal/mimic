(()=>{
 const rows=[];for(const format of ['%% %s','%c %s','%o %s','%q %s','%s %s','%d','%i','%f']){const effects=[],a={toString(){effects.push('a');return'12'}},b={toString(){effects.push('b');return'23'}};console.log(format,a,b);rows.push({format,effects})}for(const f of ['%s','%d','%i','%f'])for(const v of [12.75,-0,15n,Symbol('x')])console.log(f,v);return rows})()
