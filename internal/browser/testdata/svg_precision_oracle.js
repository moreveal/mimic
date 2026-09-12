(()=>{const ns='http://www.w3.org/2000/svg',out={},rect=b=>[b.x,b.y,b.width,b.height];
for(const [size,scale,text] of [...[1,1.001,1.1,2].flatMap(scale=>['😀','👩‍❤️‍💋‍👨','👨‍👩‍👧‍👦','👨‍👩‍👦'].map(text=>[16,scale,text])),...[1,1.001,1.1].map(scale=>[150,scale,'AV fi 012abc'])]){
 const svg=document.createElementNS(ns,'svg'),g=document.createElementNS(ns,'g'),t=document.createElementNS(ns,'text');svg.append(g);g.append(t);document.body.append(svg);t.setAttribute('x','32');t.setAttribute('y','32');t.style.font=(size===150?'italic ':'')+size+'px serif';t.style.transform='scale('+scale+')';t.textContent=text;
 try{out[size+'|'+scale+'|'+text]={box:rect(t.getBBox()),group:rect(g.getBBox()),length:t.getComputedTextLength(),char:rect(t.getExtentOfChar(0))};}catch(e){out[size+'|'+scale+'|'+text]={error:e.name,message:e.message}}svg.remove();
}return out})()
