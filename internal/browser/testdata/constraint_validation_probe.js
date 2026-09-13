(() => {
  const rows=[];
  const keys=['badInput','customError','patternMismatch','rangeOverflow','rangeUnderflow','stepMismatch','tooLong','tooShort','typeMismatch','valid','valueMissing'];
  function record(label,e){const v=e.validity;rows.push({label,tag:Object.prototype.toString.call(v),will:e.willValidate,flags:v?keys.map(k=>v[k]):null,message:e.validationMessage});}
  function make(tag,attrs={},value){const e=document.createElement(tag);for(const [k,v] of Object.entries(attrs))e.setAttribute(k,v);document.body.appendChild(e);if(value!==undefined)e.value=value;return e;}
  for(const type of ['text','email','url','number','checkbox','radio','hidden','button','submit','reset','range','color','date','time','month','week','datetime-local']){
    const e=make('input',{type,required:''});record(type+' empty',e);e.value=type==='number'?'1.5':'hello';record(type+' filled',e);e.disabled=true;record(type+' disabled',e);e.setCustomValidity('custom');record(type+' disabled custom',e);e.disabled=false;e.readOnly=true;record(type+' readonly',e);e.remove();
  }
  for(const [tag,attrs,value] of [['input',{pattern:'[a-z]+',minlength:5,maxlength:2},'ABC'],['input',{type:'number',min:2,max:5,step:2},'3'],['input',{type:'email',multiple:''},'a@b,c@d'],['textarea',{required:''},''],['select',{required:''},'']])record(JSON.stringify([tag,attrs,value]),make(tag,attrs,value));
  const e=make('input',{required:''}),v=e.validity;e.value='x';rows.push({label:'identity',same:v===e.validity,valid:v.valid});
  const events=[];e.value='';e.addEventListener('invalid',event=>{events.push([event.bubbles,event.cancelable,event.isTrusted]);event.preventDefault()});rows.push({label:'events',result:e.checkValidity(),events});
  const f=make('fieldset');f.innerHTML='<legend><input required></legend><input required>';f.disabled=true;for(const e of f.querySelectorAll('input'))record('fieldset',e);
  let illegal='';try{Object.getOwnPropertyDescriptor(ValidityState.prototype,'valid').get.call({})}catch(e){illegal=e.name}rows.push({label:'illegal',illegal});
  return rows;
})()
