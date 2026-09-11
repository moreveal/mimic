// Constructed sheets keep their parsed rules in the realm. Adoption references
// the same sheet, so replacement and rule edits are visible to every adopter.
// Snapshots serialize this state without inserting nodes into the live DOM.
const constructedStyleSheets = (() => {
  if(typeof globalThis.StyleSheet!=='function'||typeof globalThis.CSSRuleList!=='function')return {snapshot(){return []}};
  const parse = mimicSelectorLibrary.parseStylesheet, generate = mimicSelectorLibrary.generateCSS;
  let revision = 0;
  const sourceCache = new WeakMap();
  const sheets = new WeakMap(), rules = new WeakMap(), adopted = new WeakMap(), owners = new WeakMap(), ownerLists=new WeakMap();
  const requireSheet = sheet => { const state=sheets.get(sheet); if(!state)throw new TypeError('Illegal invocation'); return state; };
  const list = values => new Proxy(Object.create(globalThis.CSSRuleList.prototype), {
    get(target,key,receiver) {
      if(key==='length')return values.length;
      if(key==='item')return index=>values[Number(index)]||null;
      if(key===Symbol.iterator)return values[Symbol.iterator].bind(values);
      if(typeof key==='string'&&/^\d+$/.test(key))return values[Number(key)];
      return Reflect.get(target,key,receiver);
    }
  });
  function declarations(block) {
    const values=[];
    if(block)block.children.forEach(node=>{if(node.type==='Declaration')values.push(node)});
    return values;
  }
  const blockDeclarations=new WeakMap();
  const entriesForBlock=block=>{
    if(!block)return [];
    let entries=blockDeclarations.get(block);
    if(!entries){entries=parseCSS(declarations(block).map(node=>node.property+': '+generate(node.value).trim()+(node.important?' !important':'')+';').join(' '));blockDeclarations.set(block,entries)}
    return entries;
  };
  function declarationText(block) {return serializeCSS(entriesForBlock(block));}
  function preludeText(node) {
    if(!node)return '';
    const children=()=>Array.from(node.children,preludeText);
    if(node.type==='SelectorList'||node.type==='MediaQueryList')return children().join(', ');
    if(node.type==='AtrulePrelude'||node.type==='Condition')return children().join(' ');
    if(node.type==='Feature')return '('+node.name+(node.value?': '+generate(node.value):'')+')';
    if(node.type==='MediaQuery')return [node.modifier,node.mediaType,node.condition&&(node.mediaType?'and ':'')+preludeText(node.condition)].filter(Boolean).join(' ');
    return generate(node);
  }
  function ruleText(rule) {
    const state=rules.get(rule),node=state.node;
    if(node.type==='Rule')return preludeText(node.prelude)+' { '+declarationText(node.block)+(state.children.length?' '+state.children.map(ruleText).join(' '):'')+' }';
    const prelude=node.prelude?' '+preludeText(node.prelude):'';
    return '@'+node.name+prelude+(node.block?(state.children.length?' {\n'+state.children.map(child=>'  '+ruleText(child).replace(/\n/g,'\n  ')).join('\n')+'\n}':' { '+declarationText(node.block)+' }'):';');
  }
  function makeStyle(state) {
    const target=Object.create(CSSStyleDeclaration.prototype);
    const setText=text=>{blockDeclarations.set(state.node.block,parseCSS(String(text)));revision++};
    const names=()=>entriesForBlock(state.node.block);
    Object.defineProperties(target,{
      cssText:{get:()=>declarationText(state.node.block),set:setText,configurable:true},
      length:{get:()=>names().length,configurable:true},
      parentRule:{get:()=>state.rule,configurable:true},
      item:{value:index=>names()[Number(index)]?.name||''},
      getPropertyValue:{value:name=>readCSSDeclaration(names(),cssName(name))},
      getPropertyPriority:{value:name=>{name=cssName(name);const components=cssShorthandComponents[name]||[name];return components.every(n=>names().find(e=>e.name===n)?.priority==='important')?'important':''}},
      setProperty:{value:(name,value,priority='')=>{
        const inputName=name;name=cssName(name);if(/^webkit/i.test(name))return;value=normalizeCSSValue(name,value,inputName);if(value===null)return;priority=String(priority).toLowerCase();
        if(priority&&priority!=='important')return;
        const entries=names().slice(),components=cssShorthandComponents[name]||[name];
        if(value===''){for(let i=entries.length-1;i>=0;i--)if(components.includes(entries[i].name)||entries[i].name===name)entries.splice(i,1)}
        else for(const entry of expandCSSDeclaration({name,value,priority})){const index=entries.findIndex(e=>e.name===entry.name);if(index<0)entries.push(entry);else entries[index]=entry}
        blockDeclarations.set(state.node.block,entries);revision++;
      }},
      removeProperty:{value:name=>{const old=target.getPropertyValue(name);target.setProperty(name,'');return webkitCSSLegacyBreakShorthands.has(String(name).toLowerCase())||cssShorthandComponents[cssName(name)]?'':old;}}
    });
    return new Proxy(target,{
      get(object,key,receiver){if(typeof key==='string'&&/^\d+$/.test(key))return names()[Number(key)]?.name; if(typeof key==='string'&&!(key in object))return target.getPropertyValue(cssJSName(key));return Reflect.get(object,key,receiver)},
      set(object,key,value,receiver){if(typeof key==='string'&&key!=='cssText'&&!(key in object)){target.setProperty(cssJSInputName(key),value);return true}return Reflect.set(object,key,value,receiver)}
    });
  }
  function makeRule(node,sheet,parent=null) {
    const name=node.type==='Rule'?'CSSStyleRule':({media:'CSSMediaRule',supports:'CSSSupportsRule','font-face':'CSSFontFaceRule',keyframes:'CSSKeyframesRule',layer:node.block?'CSSLayerBlockRule':'CSSLayerStatementRule',container:'CSSContainerRule',scope:'CSSScopeRule','starting-style':'CSSStartingStyleRule',property:'CSSPropertyRule'})[node.name]||'CSSRule';
    const rule=Object.create((globalThis[name]||globalThis.CSSRule).prototype),state={node,sheet,parent,rule,children:[]};rules.set(rule,state);
    if(node.block)node.block.children.forEach(child=>{if(child.type==='Rule'||child.type==='Atrule')state.children.push(makeRule(child,sheet,rule))});
    Object.defineProperties(rule,{
      cssText:{get:()=>ruleText(rule),configurable:true},
      parentStyleSheet:{get:()=>state.sheet,configurable:true},
      parentRule:{get:()=>state.parent,configurable:true},
      type:{get:()=>node.type==='Rule'?1:({media:4,'font-face':5,keyframes:7,supports:12})[node.name]||0,configurable:true}
    });
    if(node.type==='Rule')Object.defineProperty(rule,'selectorText',{get:()=>preludeText(node.prelude),set:value=>{try{node.prelude=parse(String(value),{context:'selectorList'});revision++}catch{}},configurable:true});
    if(node.block)Object.defineProperty(rule,'style',{value:makeStyle(state),configurable:true});
    if(node.type==='Atrule'&&node.block) {
      Object.defineProperty(rule,'cssRules',{value:list(state.children),configurable:true});
      Object.defineProperty(rule,'conditionText',{get:()=>preludeText(node.prelude),configurable:true});
    }
    return rule;
  }
  function parsedRules(text,sheet) {
    const result=[];
    parse(String(text),{context:'stylesheet'}).children.forEach(node=>{
      // Constructed stylesheets cannot fetch @import rules. CSS parser recovery
      // preserves other valid rules while discarding invalid raw fragments.
      if(node.type==='Rule'&&node.prelude.type==='SelectorList'||node.type==='Atrule'&&!['import','charset'].includes(node.name))result.push(makeRule(node,sheet));
    });
    return result;
  }
  class CSSStyleSheet extends globalThis.StyleSheet {
    constructor(options={}) {
      // The generated StyleSheet constructor is deliberately illegal; returning
      // the branded derived object avoids invoking that interface constructor.
      const sheet=Object.create(new.target.prototype),state={rules:[],disabled:!!options.disabled,media:String(options.media||''),locked:false};
      sheets.set(sheet,state);state.list=list(state.rules);return sheet;
    }
    get cssRules(){const s=requireSheet(this);if(s.crossOrigin)throw new DOMException('Cannot access cross-origin stylesheet','SecurityError');return s.list}
    get rules(){return this.cssRules}
    get ownerNode(){const s=requireSheet(this);if(!s.owner)return null;return ownerSheet(s.owner)===this?s.owner:null}
    get ownerRule(){requireSheet(this);return null}
    get href(){return requireSheet(this).href||null}
    get parentStyleSheet(){requireSheet(this);return null}
    get title(){const s=requireSheet(this);return s.owner?s.owner.getAttribute('title')||null:null}
    get type(){requireSheet(this);return 'text/css'}
    get disabled(){return requireSheet(this).disabled}
    set disabled(value){requireSheet(this).disabled=!!value;revision++}
    get media(){const state=requireSheet(this);if(!state.mediaList){const media=Object.create(globalThis.MediaList.prototype);Object.defineProperties(media,{mediaText:{get:()=>state.media,set:value=>{state.media=String(value);revision++}},length:{get:()=>state.media?state.media.split(',').length:0},item:{value:index=>state.media.split(',')[Number(index)]?.trim()||''}});state.mediaList=media}return state.mediaList}
    replaceSync(text){const state=requireSheet(this);if(state.owner||state.locked)throw new DOMException('Stylesheet cannot be replaced','NotAllowedError');state.rules.splice(0,state.rules.length,...parsedRules(text,this));revision++}
    replace(text){const state=requireSheet(this);if(state.owner||state.locked)return Promise.reject(new DOMException('Stylesheet cannot be replaced','NotAllowedError'));text=String(text);state.locked=true;return Promise.resolve().then(()=>{try{state.rules.splice(0,state.rules.length,...parsedRules(text,this));revision++;return this}finally{state.locked=false}})}
    insertRule(text,index=0){const state=requireSheet(this);index=Number(index)>>>0;if(state.locked)throw new DOMException('Stylesheet is being replaced','NotAllowedError');if(index>state.rules.length)throw new DOMException('Index exceeds rule count','IndexSizeError');const ast=parse(String(text),{context:'stylesheet'});if(ast.children.size!==1)throw new DOMException('Expected one rule','SyntaxError');if(ast.children.first.type==='Atrule'&&ast.children.first.name==='import')throw new DOMException('Cannot insert @import into a constructed sheet','SyntaxError');const inserted=parsedRules(text,this);if(inserted.length!==1)throw new DOMException('Invalid rule','SyntaxError');state.rules.splice(index,0,inserted[0]);revision++;return index}
    deleteRule(index){const state=requireSheet(this);index=Number(index)>>>0;if(state.locked)throw new DOMException('Stylesheet is being replaced','NotAllowedError');if(index>=state.rules.length)throw new DOMException('Index exceeds rule count','IndexSizeError');const old=state.rules.splice(index,1)[0];rules.get(old).sheet=null;revision++}
  }
  Object.defineProperty(CSSStyleSheet.prototype,Symbol.toStringTag,{value:'CSSStyleSheet',configurable:true});
  Object.defineProperty(globalThis,'CSSStyleSheet',{value:CSSStyleSheet,writable:true,configurable:true});
  function ownerSheet(owner){
    if(!elementSlot(owner))throw new TypeError('Illegal invocation');
    const prior=owners.get(owner),tag=owner.localName,type=(owner.getAttribute('type')||'').trim().toLowerCase();
    if(!owner.isConnected||type&&type!=='text/css'||tag==='link'&&!String(owner.getAttribute('rel')||'').toLowerCase().split(/\s+/).includes('stylesheet')){owners.delete(owner);return null}
    const resource=tag==='link'?host.stylesheetResource(owner.getAttribute('href')||''):null;if(tag==='link'&&!resource){owners.delete(owner);return null}
    const source=tag==='link'?resource.body:owner.textContent||'',key=tag==='link'?resource.url:source;
    let sheet=prior?.key===key?prior.sheet:null;
    if(!sheet){sheet=new CSSStyleSheet();const s=sheets.get(sheet);s.owner=owner;s.href=resource?.url||null;s.crossOrigin=!!resource?.crossOrigin;s.rules.push(...parsedRules(source,sheet));owners.set(owner,{key,sheet})}
    const state=sheets.get(sheet);state.media=owner.getAttribute('media')||'';return sheet;
  }
  for(const type of ['HTMLStyleElement','SVGStyleElement','HTMLLinkElement'])if(globalThis[type]){
    Object.defineProperty(globalThis[type].prototype,'sheet',{get(){return ownerSheet(this)},enumerable:true,configurable:true});
    Object.defineProperty(globalThis[type].prototype,'disabled',{get(){const sheet=ownerSheet(this);return sheet?sheets.get(sheet).disabled:false},set(value){const sheet=ownerSheet(this);if(sheet){sheets.get(sheet).disabled=!!value;revision++}},enumerable:true,configurable:true});
  }
  function ownerCollection(root){let list=ownerLists.get(root);if(!list){const values=()=>compatibilitySelectors.query(root,'style,link',false,false).map(ownerSheet).filter(Boolean);list=new Proxy(Object.create(StyleSheetList.prototype),{get(target,key,receiver){const all=values();if(key==='length')return all.length;if(key==='item')return index=>values()[(+index)>>>0]||null;if(key===Symbol.iterator)return all[Symbol.iterator].bind(all);if(typeof key==='string'&&/^\d+$/.test(key))return all[Number(key)];return Reflect.get(target,key,receiver)}});ownerLists.set(root,list)}return list}
  for(const type of ['Document','ShadowRoot'])if(globalThis[type])Object.defineProperty(globalThis[type].prototype,'styleSheets',{get(){if(!(this instanceof globalThis[type]))throw new TypeError('Illegal invocation');return ownerCollection(this)},enumerable:true,configurable:true});
  // The cache is derived from the canonical CSSOM; rule edits invalidate it.
  // DOM-owned sheets are still revalidated against their current owner text.
  const sourceText=sheet=>{const state=sheets.get(sheet);if(state.disabled||state.media&&!matchMedia(state.media).matches)return '';let cached=sourceCache.get(sheet);if(!cached||cached.revision!==revision){cached={revision,text:state.rules.map(ruleText).join('\n')};sourceCache.set(sheet,cached)}return cached.text};
  function adoption(root) {
    if(!(root instanceof Document)&&!shadowSlots.has(root))throw new TypeError('Illegal invocation');
    let value=adopted.get(root);
    if(!value){value=new Proxy([],{set(target,key,sheet){if(typeof key==='string'&&/^\d+$/.test(key)&&!sheets.has(sheet))throw new TypeError('Value is not a constructed CSSStyleSheet');revision++;return Reflect.set(target,key,sheet)},deleteProperty(target,key){revision++;return Reflect.deleteProperty(target,key)}});adopted.set(root,value)}
    return value;
  }
  for(const proto of [Document.prototype,ShadowRoot.prototype])Object.defineProperty(proto,'adoptedStyleSheets',{
    configurable:true,enumerable:true,get(){return adoption(this)},set(value){const next=Array.from(value);for(const sheet of next)if(!sheets.has(sheet))throw new TypeError('Value is not a constructed CSSStyleSheet');const current=adoption(this);current.splice(0,current.length,...next)}
  });
  return {revision:()=>revision,ownerSheet,sources(root){return Array.from(ownerCollection(root)).concat(adopted.get(root)||[]).map(sourceText)},snapshot(root){return (adopted.get(root)||[]).filter(sheet=>!requireSheet(sheet).disabled).map(sheet=>{const state=requireSheet(sheet),text=state.rules.map(ruleText).join('\n');return state.media?'@media '+state.media+' {\n'+text+'\n}':text})}};
})();
