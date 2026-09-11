// CSS parsing and matching are supplied by the pinned css-select bundle.
// This adapter reads canonical wrappers; query-local memoization never outlives
// the synchronous selector operation and cannot become a second mutable DOM.
const compatibilitySelectors = (() => {
  const library = mimicSelectorLibrary;
  const astCache = new Map();
  const compiledCache = new WeakMap();
  const cacheLimit = 128;
  const extensions = new Set(['contains','icontains','matches','parent','header','selected','button','input','text','checkbox','file','password','radio','reset','image','submit']);
  const legacyElements = new Set(['before','after','first-line','first-letter']);
  const pseudoElements = new Set([...legacyElements,'selection','marker','placeholder','backdrop','file-selector-button','spelling-error','grammar-error','target-text','part','slotted','cue']);
  const emptySelector = () => [{type:'pseudo',name:'not',data:[[{type:'universal',namespace:null}]]}];
  const syntax = message => new DOMException(message, 'SyntaxError');
  // Filtering library extensions belongs at the DOM API boundary. Parsing
  // escapes, combinators, attribute operators and an+b remains upstream code.
  function validate(groups, nested = false, forgiving = false, relative = false) {
    const result = [];
    for (const group of groups) {
      try {
        let pseudoElement = false;
        const tokens = group.map((token, index) => {
          const next = {...token};
          if (token.type === 'parent' || token.type === 'attribute' && token.action === 'not') throw syntax('Non-standard selector');
          const legacy = token.type === 'pseudo' && legacyElements.has(token.name);
          if (token.type === 'pseudo-element' || legacy) {
            if (nested || !pseudoElements.has(token.name) || index !== group.length-1) throw syntax('Invalid pseudo-element');
            pseudoElement = true;
            return {...emptySelector()[0]};
          }
          if (token.type === 'pseudo') {
            if (extensions.has(token.name)) throw syntax('Non-standard pseudo-class');
            if (Array.isArray(token.data)) {
              const inner = validate(token.data, true, token.name === 'is' || token.name === 'where', token.name === 'has');
              next.data = inner.length ? inner : [emptySelector()];
            }
          }
          return next;
        });
        // Ask the library to validate unknown pseudo-classes and function args
        // even for branches that contain a non-selectable pseudo-element.
        library.compileToken(copy(tokens.length ? [tokens] : [emptySelector()]), {...options(null),relativeSelector:relative});
        if (!pseudoElement) result.push(tokens);
      } catch (error) { if (!forgiving) throw error; }
    }
    return result;
  }
  function copy(groups) {
    return groups.map(group=>group.map(token=>({...token,...(Array.isArray(token.data)?{data:copy(token.data)}:{})})));
  }
  function parsed(selector) {
    selector = String(selector);
    if (astCache.has(selector)) return astCache.get(selector);
    let value;
    try { value = validate(library.parse(selector)); }
    catch (error) { throw syntax(error.message || 'Invalid selector'); }
    if (astCache.size === cacheLimit) astCache.delete(astCache.keys().next().value);
    astCache.set(selector, value);
    return value;
  }
  let reads;
  function memo(node,key,read) {
    let record=reads.get(node);
    if (!record) reads.set(node,record=new Map());
    if (!record.has(key)) record.set(key,read());
    return record.get(key);
  }
  let documentID=host.documentRootID();bootstrapRestoreHooks.push(()=>{documentID=host.documentRootID()});
  const children = node => memo(node,'children',()=>{
    const slot=elementSlot(node);
    if(node===document||slot)return host.nodeChildren(node===document?documentID:slot.nodeId).map(wrap);
    return fragmentSlots.get(node)?.children.slice()||[];
  });
  const parent = node => memo(node,'parent',()=>{
    if(syntheticParents.has(node))return syntheticParents.get(node);
    const slot=elementSlot(node);return slot?wrap(host.parentNode(slot.nodeId)):null;
  });
  const attribute = (node,name) => memo(node,'attribute:'+name,()=>host.getAttribute(elementSlot(node).nodeId,name)??undefined);
  const adapter = {
    isTag: node => elementSlot(node)?.type === 'element',
    getName: node => elementSlot(node).tagName.toLowerCase(),
    getChildren: children,
    getParent: parent,
    getSiblings: node => parent(node) ? children(parent(node)) : [node],
    getAttributeValue: attribute,
    hasAttrib: (node,name) => attribute(node,name)!==undefined,
    getText: node => memo(node,'text',()=>node.textContent),
    equals: (a,b) => a===b
  };
  const statePseudos={
    modal:node=>compatibilityElementState.modal(node),
    target:node=>elementSlot(node).nodeId===host.selectorTargetID(),
    defined:node=>compatibilityElementState.isDefined(node),
    focus:node=>compatibilityElementState.focused()===node,
    'focus-within':node=>{for(let current=compatibilityElementState.focused();current;current=parent(current))if(current===node)return true;return false;},
    'focus-visible':node=>compatibilityElementState.focusVisible(node)
  };
  function options(scope) {
    return {adapter,equals:adapter.equals,context:scope,relativeSelector:false,cacheResults:false,pseudos:statePseudos};
  }
  function predicate(scope, selector) {
    const ast = parsed(selector);
    let cache=compiledCache.get(scope);
    if (!cache) compiledCache.set(scope,cache=new Map());
    if (cache.has(selector)) return cache.get(selector);
    let matcher;
    try { matcher=ast.length?library.compileToken(copy(ast),options(scope)):()=>false; }
    catch (error) { throw syntax(error.message || 'Invalid selector'); }
    if (cache.size===cacheLimit) cache.delete(cache.keys().next().value);
    cache.set(selector,matcher);
    return matcher;
  }
  function run(callback) {
    const previous=reads;
    reads=new WeakMap();
    try { return callback(); } finally { reads=previous; }
  }
  const scopeFor = root => root === document ? document.documentElement : root;
  function simpleNative(selector) {
    const groups=parsed(selector);
    const leaves=groups.map(simpleNativeGroup);
    return leaves.length&&leaves.every(Boolean)?leaves.join(','):null;
  }
  function simpleNativeGroup(tokens) {
    // Preserve exactly the legacy native leaf grammar. Other compounds use
    // the same upstream matcher as structural selectors.
    if(tokens.length>2||tokens.length===2&&!(tokens[0].type==='tag'&&tokens[1].type==='attribute'&&tokens[1].name==='class'&&tokens[1].action==='element'))return null;
    let source='';
    const identifier=value=>/^[a-zA-Z0-9_-]+$/.test(value);
    for(const token of tokens) {
      if(token.type==='universal'&&token.namespace===null)source+='*';
      else if(token.type==='tag'&&token.namespace===null&&identifier(token.name))source+=token.name;
      else if(token.type==='attribute'&&token.namespace===null&&token.ignoreCase==='quirks'&&identifier(token.value)&&
          (token.name==='id'&&token.action==='equals'||token.name==='class'&&token.action==='element'))source+=(token.name==='id'?'#':'.')+token.value;
      else if(tokens.length===1&&token.type==='attribute'&&token.namespace===null&&identifier(token.name)&&token.action==='exists')source='['+token.name+']';
      else if(tokens.length===1&&token.type==='attribute'&&token.namespace===null&&identifier(token.name)&&token.action==='equals'&&
          (token.ignoreCase===false||token.ignoreCase===null&&!library.caseInsensitiveAttributes.has(token.name.toLowerCase()))&&/^[a-zA-Z0-9_.-]*$/.test(token.value))source='['+token.name+'="'+token.value+'"]';
      else return null;
    }
    return source||null;
  }
  function query(root,selector,first=false,records=false) {
    selector=String(selector);
    const simple=simpleNative(selector),slot=elementSlot(root);
    if(simple&&(root===document||slot)) {
      const id=root===document?documentID:slot.nodeId;
      if(first)return wrap(host.queryWithin(id,simple));
      const ids=host.queryAllWithin(id,simple);return records?ids:ids.map(wrap);
    }
    return run(()=>{
      const scope=scopeFor(root), match=predicate(scope,selector);
      const result=(first?library.findOne:library.findAll)(match,children(root),options(scope));
      return records?result.map(node=>elementSlot(node)):result;
    });
  }
  function matches(node,selector) {
    selector=String(selector);
    const simple=simpleNative(selector);
    if(simple)return host.matches(elementSlot(node).nodeId,simple);
    return run(()=>predicate(node,selector)(node));
  }
  // Stylesheet rules outlive individual style reads. Compile each immutable
  // selector once, without caching DOM match results or retaining an element
  // as its scope. Explicit :scope selectors keep their per-element semantics.
  function compileStyle(selector) {
    try {
      // A style read matches many rules against the same element. Keep even
      // leaf predicates in the shared read scope so id/class/attribute facts
      // cross the host boundary once, not once for every stylesheet rule.
      const ast=parsed(selector);
      const scoped=groups=>groups.some(group=>group.some(token=>token.type==='pseudo'&&token.name==='scope'||Array.isArray(token.data)&&scoped(token.data)));
      if(scoped(ast))return node=>predicate(node,selector)(node);
      return ast.length?library.compileToken(copy(ast),options(null)):()=>false;
    } catch(error) {
      // Invalid/unsupported CSS rules are ignored, but DOM selector methods
      // continue to throw SyntaxError through their existing boundary.
      if(error?.name==='SyntaxError')return ()=>false;
      throw error;
    }
  }
  const matchingStyles=(node,rules)=>run(()=>rules.filter(rule=>rule.matches(node)));
  function closest(node,selector) {
    selector=String(selector);
    return run(()=>{
      const match=predicate(node,selector);
      for(let current=node;current;current=parent(current)) if(adapter.isTag(current)&&match(current)) return current;
      return null;
    });
  }
  function getElementById(root,id) {
    id=String(id);
    if(!id)return null;
    const slot=elementSlot(root);
    if(root===document||slot){const found=host.elementByID(root===document?documentID:slot.nodeId,id);return found?wrap(found):null;}
    return run(()=>library.findOne(node=>attribute(node,'id')===id,children(root),options(scopeFor(root))));
  }
  return {query,matches,closest,getElementById,compileStyle,matchingStyles};
})();
// These late semantic replacements are WebIDL operations too. Validate the
// private brand before arity and conversion, outside selector-parser error
// handling, so conversion exceptions keep their identity and caller realm.
const installSelectorOperation = (prototype,name,operation) => {
  const fn = {[name](selector){
    const slot=elementSlot(this);
    const valid=prototype===Document.prototype ? this===document||slot?.type==='document' : slot?.type==='element';
    if(!valid)throw new TypeError('Illegal invocation');
    if(arguments.length===0)throw new TypeError('Not enough arguments');
    if(typeof selector==='symbol')throw new TypeError('Cannot convert a Symbol value to a string');
    return operation(this,String(selector));
  }}[name];
  markNative(fn,name);
  Object.defineProperty(prototype,name,{value:fn,writable:true,enumerable:true,configurable:true});
};
for (const prototype of [Document.prototype,Element.prototype]) {
  installSelectorOperation(prototype,'querySelector',(receiver,selector)=>compatibilitySelectors.query(receiver,selector,true));
  installSelectorOperation(prototype,'querySelectorAll',(receiver,selector)=>nodeList(compatibilitySelectors.query(receiver,selector,false,true)));
}
for(const name of ['matches','webkitMatchesSelector','closest'])
  installSelectorOperation(Element.prototype,name,(receiver,selector)=>compatibilitySelectors[name==='webkitMatchesSelector'?'matches':name](receiver,selector));
installSelectorOperation(Document.prototype,'getElementById',(receiver,id)=>compatibilitySelectors.getElementById(receiver,id));
// Synthetic DocumentFragments still lack a cross-realm private-brand bridge.
// Keep their existing bindings until borrowed calls can be validated without
// rejecting genuine foreign fragments or accepting prototype forgeries.
Object.defineProperties(DocumentFragment.prototype,{
  querySelector:{value:function(selector){return compatibilitySelectors.query(this,selector,true);},writable:true,configurable:true},
  querySelectorAll:{value:function(selector){return nodeList(compatibilitySelectors.query(this,selector,false,true));},writable:true,configurable:true},
  getElementById:{value:function(id){return compatibilitySelectors.getElementById(this,id);},writable:true,configurable:true}
});

// CDP calls the same closed-over engine instead of evaluating page-controlled
// querySelector properties or maintaining an independent native CSS grammar.
registerBootstrapCallback('setDOMQueryCallback',(nodeID,selector,all)=>{
  try {
    const root=nodeID===host.documentRootID()?document:wrap(nodeID);
    const result=compatibilitySelectors.query(root,selector,!all);
    return JSON.stringify({ids:(all?result:result?[result]:[]).map(node=>elementSlot(node).nodeId)});
  } catch(error) {return JSON.stringify({error:String(error.name)+': '+String(error.message)}); }
});
