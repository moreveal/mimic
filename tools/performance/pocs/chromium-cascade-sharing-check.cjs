const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

const source = fs.readFileSync('internal/webapi/surface.js', 'utf8');
const start = source.indexOf('  const uncachedCSSDeclarations =');
const end = source.indexOf('  // This private declaration array', start);
let calls = 0;
const rule = {
  order: 0,
  specificity: 1,
  declarations() {
    calls++;
    return [{ name: 'color', value: 'red', priority: '' }];
  },
};
const rules = [rule];
const state = {
  rules,
  styleReadCache: { retainable: false },
  styleCascades: new WeakMap(),
  compatibilitySelectors: { matchingStyles: (_, program) => program },
  styleSheetRules: () => state.rules,
  cssObservationNodeState: (element) => ({ inline: element.inline || 'j[]' }),
  elementSlot: (element) => element,
  host: { getAttribute: (id) => (id === 1 ? '' : null), systemFonts: () => ({}) },
  cssComputedNames: [],
  cssInitialValues: new Map(),
  parseCSS: () => [],
  parseCSSFont: () => null,
  cssShorthandComponents: { font: [] },
};
vm.createContext(state);
vm.runInContext(
  source.slice(start, end) + '\nglobalThis.compute = uncachedCSSDeclarations;',
  state,
);
const a = { tagName: 'DIV' };
const b = { tagName: 'SPAN' };
const first = state.compute(a);
assert.equal(state.compute(b), first, 'equivalent specified cascades share');
assert.equal(calls, 1);
assert.notEqual(state.compute({ tagName: 'IFRAME' }), first, 'UA defaults differ');
assert.notEqual(state.compute({ tagName: 'DIALOG', nodeId: 1 }), first);
assert.notEqual(state.compute({ tagName: 'DIALOG', nodeId: 2 }), first);
assert.notEqual(state.compute(b, 'before'), first);
assert.notEqual(state.compute({ inline: 'j[{"name":"color","value":"blue"}]' }), first);
state.rules = [rule];
assert.notEqual(state.compute(b), first, 'rule-program identity partitions ordinals');
state.rules = rules;
state.styleReadCache = { retainable: false };
assert.notEqual(state.compute(b), first, 'observation reset drops sharing');
state.styleReadCache.sharedSpecifiedCascades.entries = 16384;
const capped = state.compute({ inline: 'j[{"name":"color","value":"green"}]' });
assert.notEqual(state.compute({ inline: 'j[{"name":"color","value":"green"}]' }), capped);
assert.equal(state.styleReadCache.sharedSpecifiedCascades.entries, 16384);
state.rules = [
  {
    order: 0,
    specificity: 1,
    declarations: () => [
      { name: 'font-size', value: '', pending: { systemFont: true, value: 'menu' } },
    ],
  },
];
state.styleReadCache = { retainable: false };
assert.notEqual(state.compute(a), state.compute(b), 'system fonts not shared');
console.log('PASS: exact cascade inputs, observation reset, capacity, system-font exclusion');
