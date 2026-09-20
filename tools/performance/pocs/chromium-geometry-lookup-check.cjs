const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(
  path.join(__dirname, '../../../internal/webapi/css_box_geometry.js'),
  'utf8',
);
const start = source.indexOf('  const size = (element) => {');
const end = source.indexOf('  const rect = (element) => {', start);
assert(start >= 0 && end > start);
const original = source.slice(start, end);
const needle = "    if (active?.state === 'computing') return active.value;";
assert(original.includes(needle));
const candidate = original.replace(
  needle,
  `${needle}\n    if (cache.has(element)) return cache.get(element);`,
);

function run(program, scenario) {
  const target = {};
  const parent = {};
  const box = { width: 37, height: 19 };
  const boxSizes = new WeakMap();
  const sizePlans = new WeakMap();
  let ancestorReads = 0;
  if (scenario === 'computing') {
    sizePlans.set(target, { state: 'computing', value: box });
  } else if (scenario !== 'late-table-box') {
    boxSizes.set(target, box);
  }
  boxSizes.set(parent, { width: 100, height: 100 });
  const context = {
    styleReadCache: { boxSizes, sizePlans },
    geometryParent(node) {
      ancestorReads++;
      if (scenario === 'late-table-box' && node === target) {
        boxSizes.set(target, box);
      }
      return node === target ? parent : null;
    },
    state(node) {
      assert.equal(node, parent);
      return { display: scenario === 'normal' ? 'block' : 'table' };
    },
    target,
  };
  vm.createContext(context);
  const result = vm.runInContext(`${program}\nsize(target);`, context);
  assert.equal(result, box);
  return ancestorReads;
}

for (const scenario of ['normal', 'table', 'computing', 'late-table-box']) {
  const before = run(original, scenario);
  const after = run(candidate, scenario);
  if (scenario === 'normal' || scenario === 'table') {
    assert(before > 0);
    assert.equal(after, 0);
  } else {
    assert.equal(after, before);
  }
  console.log(`${scenario}: same result identity; ancestor reads ${before} -> ${after}`);
}
