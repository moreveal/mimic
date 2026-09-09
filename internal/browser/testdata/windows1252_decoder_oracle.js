(()=>{
 const bytes=Uint8Array.from({length:256},(_,i)=>i),points=s=>Array.from(s,c=>c.codePointAt(0));
 const aliases=['windows-1252','ascii','us-ascii','iso-8859-1','iso8859-1','iso88591','iso_8859-1','iso_8859-1:1987','latin1','l1','cp819','ibm819','csisolatin1','x-cp1252','ansi_x3.4-1968','cp1252','iso-ir-100'];
 const result={points:points(new TextDecoder('windows-1252').decode(bytes)),aliases:aliases.map(label=>({label,encoding:new TextDecoder(label).encoding,match:new TextDecoder(label).decode(bytes)===new TextDecoder('windows-1252').decode(bytes)})),fatal:points(new TextDecoder('windows-1252',{fatal:true}).decode(Uint8Array.of(0x81,0x8d,0x8f,0x90,0x9d))),bom:points(new TextDecoder('windows-1252').decode(Uint8Array.of(0xef,0xbb,0xbf)))};
 const stream=new TextDecoder('windows-1252');result.stream=[points(stream.decode(Uint8Array.of(0x80,0xef),{stream:true})),points(stream.decode(Uint8Array.of(0xbb),{stream:true})),points(stream.decode(Uint8Array.of(0xbf))),points(stream.decode())];
 const raw=Uint8Array.of(0,0x80,0x81,255);result.view=points(new TextDecoder('latin1').decode(new DataView(raw.buffer,1,2)));result.slice=points(new TextDecoder('latin1').decode(raw.subarray(1,3)));
 result.labels=[];for(const label of ['\t\n\f\r windows-1252 \t','\u00a0windows-1252\u00a0','\u000bwindows-1252\u000b']){try{result.labels.push({label,encoding:new TextDecoder(label).encoding})}catch(e){result.labels.push({label,error:e.name})}}
 return result;
})()
