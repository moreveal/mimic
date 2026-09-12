// Local Date operations use the Context zone, never process-global ICU/TZ.
// Native Date objects still own the time value (including subclasses, cloning,
// UTC operations and invalid dates). Cache only immutable transition intervals.
if(nativeIntl){
  const {DateTimeFormat,NumberFormat,Collator}=Intl;
  const NativeDate=Date, proto=Date.prototype;
  const getTime=Function.prototype.call.bind(proto.getTime);
  const setTime=Function.prototype.call.bind(proto.setTime);
  const nativeParse=Date.parse;
  const utc={};
  for(const suffix of ['FullYear','Month','Date','Day','Hours','Minutes','Seconds','Milliseconds']){
    utc['get'+suffix]=Function.prototype.call.bind(proto['getUTC'+suffix]);
    if(suffix!=='Day')utc['set'+suffix]=Function.prototype.call.bind(proto['setUTC'+suffix]);
  }
  let intervals=[],zoneNameFormat,defaultNumberFormat,temporalZone;
  const defaultDateFormats=new Map();
  bootstrapRestoreHooks.push(()=>{intervals=[];zoneNameFormat=undefined;defaultNumberFormat=undefined;temporalZone=undefined;defaultDateFormats.clear()});
  const zone=ms=>{
    if(!Number.isFinite(ms))return [NaN];
    for(const interval of intervals)if(ms>=interval[1]&&ms<interval[2])return interval;
    const interval=host.dateZone(ms);
    if(!interval)return [NaN];
    if(intervals.length===8)intervals.shift();
    intervals.push(interval);return interval;
  };
  const calendarCycle=12622780800000,yearShifts=new WeakMap();
  const fullYear=utc.getFullYear;
  utc.getFullYear=d=>fullYear(d)+(yearShifts.get(d)||0);
  const wallTime=d=>getTime(d)+(yearShifts.get(d)||0)/400*calendarCycle;
  const local=ms=>{
    const wall=ms+zone(ms)[0],shift=Math.abs(wall)>8.64e15?Math.sign(wall)*400:0;
    const d=new NativeDate(wall-shift/400*calendarCycle);
    if(shift)yearShifts.set(d,shift);
    return d;
  };
  const universal=wall=>{
    if(!Number.isFinite(wall))return NaN;
    // Try the offsets on both sides. For repeated times select the earlier
    // instant; for gaps use the offset before the forward transition (Chrome's
    // compatible disambiguation). Includes non-hour and date-line changes.
    const offsets=new Set([zone(wall-172800000)[0],zone(wall)[0],zone(wall+172800000)[0]]);
    let first=Infinity,after=Infinity;
    for(const offset of offsets){
      const candidate=wall-offset,actual=candidate+zone(candidate)[0];
      if(actual===wall)first=Math.min(first,candidate);
      else if(actual>wall)after=Math.min(after,candidate);
    }
    return Number.isFinite(first)?first:Number.isFinite(after)?after:NaN;
  };
  const primitive=value=>{
    if(value===null||typeof value!=='object'&&typeof value!=='function')return value;
    const exotic=value[Symbol.toPrimitive];
    if(exotic!==undefined&&exotic!==null){const result=Reflect.apply(exotic,value,['default']);if(Object(result)!==result)return result;throw new TypeError('Cannot convert object to primitive value')}
    for(const name of ['valueOf','toString'])if(typeof value[name]==='function'){const result=value[name]();if(Object(result)!==result)return result}
    throw new TypeError('Cannot convert object to primitive value');
  };
  const parse=text=>{
    // ISO date-only is UTC. Explicit zones are native syntax; unzoned date-time
    // and legacy local strings are parsed against UTC, then resolved in context.
    const s=bindingString(text),trim=s.trim();
    if(/^(?:\d{4}|[+-]\d{6})(?:-\d\d(?:-\d\d)?)?$/.test(trim)||/(?:z|gmt|utc|[epmc][sd]t)\b/i.test(trim)||/[t\s]\d[^]*[+-]\d\d(?::?\d\d)?(?:\s*\([^]*\))?$/i.test(trim))return nativeParse(s);
    const iso=/^(?:\d{4}|[+-]\d{6})-\d\d-\d\dT/.test(trim);
    return universal(nativeParse(trim+(iso?'Z':' UTC')));
  };
  const componentTime=args=>{
    const values=args.slice(0,7).map(value=>+value);
    let year=values[0];if(Number.isFinite(year)&&Math.trunc(year)>=0&&Math.trunc(year)<=99)year=1900+Math.trunc(year);
    const d=new NativeDate(0);
    utc.setFullYear(d,year,values[1],values.length>2?values[2]:1);
    utc.setHours(d,values.length>3?values[3]:0,values.length>4?values[4]:0,values.length>5?values[5]:0,values.length>6?values[6]:0);
    return universal(getTime(d));
  };
  const MimicDate=new Proxy(NativeDate,{
    apply(){return proto.toString.call(new NativeDate())},
    construct(target,args,newTarget){
      if(!args.length)return Reflect.construct(target,args,newTarget);
      let ms;
      if(args.length>1)ms=componentTime(args);
      else {
        // [[DateValue]] precedes ToPrimitive, even for foreign realm Dates.
        try{ms=getTime(args[0])}catch{const value=primitive(args[0]);ms=typeof value==='string'?parse(value):+value}
      }
      return Reflect.construct(target,[ms],newTarget);
    }
  });
  const install=(name,operation)=>{const value=({[name](...args){return Reflect.apply(operation,this,args)}})[name];Object.defineProperty(value,'length',{value:name.startsWith('toLocale')?0:operation.length});markNative(value,name);Object.defineProperty(proto,name,{value,writable:true,configurable:true})};
  for(const suffix of ['FullYear','Month','Date','Day','Hours','Minutes','Seconds','Milliseconds']){
    install('get'+suffix,function(){return utc['get'+suffix](local(getTime(this)))});
    if(suffix!=='Day'){
      const counts={FullYear:3,Month:2,Date:1,Hours:4,Minutes:3,Seconds:2,Milliseconds:1};
      const setter=function(value){
        const ms=getTime(this),values=Array.from(arguments).slice(0,counts[suffix]).map(v=>+v);
        const date=Number.isNaN(ms)&&suffix==='FullYear'?new NativeDate(0):local(ms);
        if(!values.length)values.push(NaN);
        if(suffix==='FullYear'&&yearShifts.has(date))values[0]-=yearShifts.get(date);
        utc['set'+suffix](date,...values);
        return setTime(this,universal(wallTime(date)));
      };
      Object.defineProperty(setter,'length',{value:counts[suffix]});install('set'+suffix,setter);
    }
  }
  install('getYear',function(){return utc.getFullYear(local(getTime(this)))-1900});
  install('setYear',function(year){const ms=getTime(this);year=+year;const d=Number.isNaN(ms)?new NativeDate(0):local(ms);if(Number.isFinite(year)&&Math.trunc(year)>=0&&Math.trunc(year)<=99)year=1900+Math.trunc(year);utc.setFullYear(d,year-(yearShifts.get(d)||0));return setTime(this,universal(wallTime(d)))});
  install('getTimezoneOffset',function(){const offset=zone(getTime(this))[0];return offset===0?0:Math.trunc(-offset/60000)});
  const pad=(n,length=2)=>String(n).padStart(length,'0');
  const dateString=ms=>{const d=local(ms),y=utc.getFullYear(d);return ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'][utc.getDay(d)]+' '+['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'][utc.getMonth(d)]+' '+pad(utc.getDate(d))+' '+(y<0?'-':'')+pad(Math.abs(y),4)};
  const timeString=ms=>{
    const d=local(ms),offset=zone(ms)[0]/60000,abs=Math.abs(offset);
    zoneNameFormat??=new DateTimeFormat(intlEnvironment.locale,{timeZone:intlEnvironment.timeZone,timeZoneName:'long'});
    // V8 DateCache::LocalTimezone maps out-of-int32 epochs to an equivalent
    // calendar year for the display name only; historical offsets stay real.
    // https://github.com/v8/v8/blob/main/src/date/date.h#L82-L135
    let nameTime=ms;
    if(ms<0||ms>2147483647000){
      const equivalent=new NativeDate(ms),year=utc.getFullYear(equivalent),jan=new NativeDate(0);
      // January itself may lie outside TimeClip at the minimum Date. A
      // Gregorian 400-year cycle preserves its weekday without clipping.
      utc.setFullYear(jan,2000+((year%400)+400)%400,0,1);
      const leap=year%4===0&&(year%100!==0||year%400===0);
      const base=(leap?1956:1967)+(utc.getDay(jan)*12)%28;
      utc.setFullYear(equivalent,2008+(base+84-2008)%28);
      nameTime=getTime(equivalent);
    }
    const name=zoneNameFormat.formatToParts(nameTime).find(part=>part.type==='timeZoneName')?.value;
    return pad(utc.getHours(d))+':'+pad(utc.getMinutes(d))+':'+pad(utc.getSeconds(d))+' GMT'+(offset<0?'-':'+')+pad(Math.floor(abs/60))+pad(Math.floor(abs%60))+' ('+name+')';
  };
  install('toDateString',function(){const ms=getTime(this);return Number.isNaN(ms)?'Invalid Date':dateString(ms)});
  install('toTimeString',function(){const ms=getTime(this);return Number.isNaN(ms)?'Invalid Date':timeString(ms)});
  install('toString',function(){const ms=getTime(this);return Number.isNaN(ms)?'Invalid Date':dateString(ms)+' '+timeString(ms)});
  for(const name of ['toLocaleString','toLocaleDateString','toLocaleTimeString']){
    install(name,function(locales,options){
      const ms=getTime(this);if(Number.isNaN(ms))return 'Invalid Date';
      if(options===null)throw new TypeError('Cannot convert null to object');
      const opts=Object.create(options===undefined?null:Object(options));
      const date=name!=='toLocaleTimeString',time=name!=='toLocaleDateString';
      if(name==='toLocaleDateString'&&opts.timeStyle!==undefined||name==='toLocaleTimeString'&&opts.dateStyle!==undefined)throw new TypeError('Invalid style option');
      const fields=[...(date?['weekday','year','month','day']:[]),...(time?['dayPeriod','hour','minute','second','fractionalSecondDigits']:[])];
      if(fields.every(key=>opts[key]===undefined)&&opts.dateStyle===undefined&&opts.timeStyle===undefined){
        for(const key of [...(date?['year','month','day']:[]),...(time?['hour','minute','second']:[])])Object.defineProperty(opts,key,{value:'numeric',configurable:true,enumerable:true});
      }
      if(locales===undefined&&options===undefined){
        let formatter=defaultDateFormats.get(name);
        if(!formatter){formatter=new DateTimeFormat(undefined,opts);defaultDateFormats.set(name,formatter)}
        return formatter.format(ms);
      }
      return new DateTimeFormat(locales,opts).format(ms);
    });
  }
  const parseDate=({parse(string){return parse(string)}}).parse;markNative(parseDate,'parse');
  Object.defineProperty(MimicDate,'parse',{value:parseDate,writable:true,configurable:true});
  Object.defineProperty(proto,'constructor',{value:MimicDate,writable:true,configurable:true});
  globalThis.Date=MimicDate;
  for(const Constructor of [Number,BigInt]){
    const valueOf=Function.prototype.call.bind(Constructor.prototype.valueOf);
    const method=({toLocaleString(locales,options){const value=valueOf(this);const formatter=locales===undefined&&options===undefined?(defaultNumberFormat??=new NumberFormat()):new NumberFormat(locales,options);return formatter.format(value)}}).toLocaleString;
    Object.defineProperty(method,'length',{value:0});
    markNative(method,'toLocaleString');Object.defineProperty(Constructor.prototype,'toLocaleString',{value:method,writable:true,configurable:true});
  }
  const compare=({localeCompare(that,locales,options){if(this==null)throw new TypeError('Invalid receiver');const left=bindingString(this),right=bindingString(that);return new Collator(locales,options).compare(left,right)}}).localeCompare;
  Object.defineProperty(compare,'length',{value:1});markNative(compare,'localeCompare');Object.defineProperty(String.prototype,'localeCompare',{value:compare,writable:true,configurable:true});
  const canonicalLocales=Intl.getCanonicalLocales;
  for(const name of ['toLocaleLowerCase','toLocaleUpperCase']){
    const native=String.prototype[name];
    const method=({[name](locales){if(this==null)throw new TypeError('Invalid receiver');const value=bindingString(this),list=canonicalLocales(locales);return Reflect.apply(native,value,list.length?[list]:[intlEnvironment.locale])}})[name];
    Object.defineProperty(method,'length',{value:0});markNative(method,name);Object.defineProperty(String.prototype,name,{value:method,writable:true,configurable:true});
  }
  if(typeof Temporal!=='undefined'){
    const now=Temporal.Now;
    const defaultZone=()=>temporalZone??=new DateTimeFormat().resolvedOptions().timeZone;
    const timeZoneId=({timeZoneId(){return defaultZone()}}).timeZoneId;
    markNative(timeZoneId,'timeZoneId');Object.defineProperty(now,'timeZoneId',{value:timeZoneId,writable:true,configurable:true});
    for(const name of ['zonedDateTimeISO','plainDateTimeISO','plainDateISO','plainTimeISO']){
      const native=now[name];if(typeof native!=='function')continue;
      const method=({[name](zone){return Reflect.apply(native,this,[zone===undefined?defaultZone():zone])}})[name];
      Object.defineProperty(method,'length',{value:native.length});markNative(method,name);Object.defineProperty(now,name,{value:method,writable:true,configurable:true});
    }
    for(const name of ['Instant','ZonedDateTime','PlainDateTime','PlainDate','PlainTime','PlainYearMonth','PlainMonthDay','Duration']){
      const proto=Temporal[name]?.prototype,native=proto?.toLocaleString;if(typeof native!=='function')continue;
      const method=({toLocaleString(locales,options){
        const list=[...canonicalLocales(locales),intlEnvironment.locale];
        if(name==='Instant'&&options!==null)options=new Proxy(options===undefined?{}:Object(options),{get(target,key){const value=Reflect.get(target,key,target);return key==='timeZone'&&value===undefined?intlEnvironment.timeZone:value}});
        return Reflect.apply(native,this,[list,options]);
      }}).toLocaleString;
      Object.defineProperty(method,'length',{value:native.length});markNative(method,'toLocaleString');Object.defineProperty(proto,'toLocaleString',{value:method,writable:true,configurable:true});
    }
  }
}
