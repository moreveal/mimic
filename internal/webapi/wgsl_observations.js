// A bounded WGSL front end for arithmetic render observations. Lowering shares
// the scalar/vector evaluator with WebGL; unsupported syntax is never executed
// as JavaScript and is reported explicitly by the resource layer.
const wgslObservations=(()=>{
 const fail=message=>{const e=new Error(message);e.unsupported=true;throw e};
 const typeName=t=>({f32:'float',i32:'int',u32:'int',bool:'bool',vec2f:'vec2',vec3f:'vec3',vec4f:'vec4',vec2i:'ivec2',vec3i:'ivec3',vec4i:'ivec4'}[t]||t);
 function compile(source){
  if(source.length>65536)fail('WGSL source limit');source=source.replace(/\/\*[\s\S]*?\*\//g,'').replace(/\/\/[^\n]*/g,'');
  const tokens=[],re=/\s+|(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?[fiu]?|[A-Za-z_]\w*|->|<=|>=|==|!=|&&|\|\||[{}()[\];,:.@+*\/%!<>=-]/gy;let pos=0;
  while(pos<source.length){re.lastIndex=pos;const m=re.exec(source);if(!m)fail('WGSL token');pos=re.lastIndex;if(!/^\s+$/.test(m[0]))tokens.push(m[0])}
  let at=0;const peek=()=>tokens[at],take=()=>tokens[at++],expect=t=>{if(take()!==t)throw new SyntaxError('Expected '+t)},identifier=()=>{const t=take();if(!/^[A-Za-z_]\w*$/.test(t||''))throw new SyntaxError('Expected identifier');return t};
  const attributes=()=>{const a={};while(peek()==='@'){take();const name=identifier();let value=true;if(peek()==='('){take();value=take();expect(')')}a[name]=value}return a};
  function type(){let name=identifier();if(peek()==='<'){take();const inner=type();if(name==='array'){expect(',');const size=Number(take());expect('>');if(!Number.isInteger(size)||size<1||size>256)fail('WGSL array size');return {array:inner,size}}expect('>');if(/^vec[234]$/.test(name))name+=(inner==='float'?'f':inner==='int'?'i':'?');else fail('WGSL generic type')}return typeName(name)}
  const precedence={'||':1,'&&':2,'==':3,'!=':3,'<':4,'>':4,'<=':4,'>=':4,'+':5,'-':5,'*':6,'/':6,'%':6};
  function expression(min=0){let node,t=take();if(t==='('){node=expression();expect(')')}else if(['+','-','!'].includes(t))node={op:'unary',kind:t,value:expression(7)};else if(/^(\d|\.)/.test(t||''))node={op:'literal',value:Math.fround(Number(t.replace(/[fiu]$/,'')))};else if(t==='true'||t==='false')node={op:'literal',value:t==='true'};else if(/^[A-Za-z_]\w*$/.test(t||'')){
    let arrayType;if(t==='array'&&peek()==='<'){at--;arrayType=type()}
    if(peek()==='('){take();const args=[];if(peek()!==')')do{args.push(expression());if(peek()!==',')break;take()}while(peek()!==')');expect(')');if(arrayType){if(args.length!==arrayType.size)throw new SyntaxError('Array constructor size');node={op:'array',values:args}}else node={op:'call',name:typeName(t),args}}
    else node={op:'variable',name:t};
   }else throw new SyntaxError('WGSL expression');
   while(peek()==='.'||peek()==='['){if(take()==='.'){const fields=identifier();if(!/^[xyzwrgba]{1,4}$/.test(fields))fail('WGSL member');node={op:'swizzle',value:node,fields}}else{const index=expression();expect(']');node={op:'index',value:node,index}}}
   while(Object.hasOwn(precedence,peek())&&precedence[peek()]>=min){const kind=take();node={op:'binary',kind,left:node,right:expression(precedence[kind]+1)}}return node;
  }
  function statement(){if(peek()==='{'){take();const body=[];while(peek()!=='}'){if(at>=tokens.length)throw new SyntaxError('Unclosed WGSL block');body.push(statement())}take();return {op:'block',body}}
   if(peek()==='return'){take();const value=expression();expect(';');return {op:'returnValue',value}}
   if(peek()==='if'){take();const test=expression(),yes=statement();let no;if(peek()==='else'){take();no=statement()}return {op:'if',test,yes,no}}
   if(peek()==='discard'){take();expect(';');return {op:'discard'}}
   if(['var','let','const'].includes(peek())){const constant=take()!=='var',name=identifier();let declaredType;if(peek()===':'){take();declaredType=type()}expect('=');const value=expression();expect(';');return {op:'declare',name,type:declaredType||'float',value,constant}}
   const name=identifier();expect('=');const value=expression();expect(';');return {op:'assign',name,kind:'=',value};
  }
  const functions=new Map();while(at<tokens.length){const attrs=attributes();if(attrs.compute)fail('WGSL compute execution');expect('fn');const name=identifier();expect('(');const inputs=[];while(peek()!==')'){const attr=attributes(),name=identifier();expect(':');inputs.push({name,type:type(),attr});if(peek()!==',')break;take()}expect(')');expect('->');const outputAttributes=attributes(),outputType=type(),main=statement();if(functions.has(name))throw new SyntaxError('Duplicate WGSL function');functions.set(name,{name,attrs,inputs,outputAttributes,outputType,main,globals:[],output:'__result'})}return functions;
 }
 function execute(fn,builtins){const inputs={};for(const p of fn.inputs){if(!p.attr.builtin||!(p.attr.builtin in builtins))fail('WGSL stage input');inputs[p.name]=builtins[p.attr.builtin]}return glslObservations.execute(fn,inputs)}
 return {compile,execute};
})();
