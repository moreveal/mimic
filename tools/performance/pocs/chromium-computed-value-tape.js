// Diagnostic fixture replay, never a browser implementation. Install this
// beside cssComputedValue, and call it only on owner computed-cache misses.
let diagnosticComputedTape;
let diagnosticComputedTapeStats;
let diagnosticComputedTapeDecoded;
bootstrapRestoreHooks.push(() => {
  diagnosticComputedTape = undefined;
  diagnosticComputedTapeStats = undefined;
  diagnosticComputedTapeDecoded = undefined;
});
const diagnosticComputedLayerEnabled = (layer) => {
  diagnosticComputedTape ||= JSON.parse(host.diagnosticComputedTape('read', '', ''));
  return (
    diagnosticComputedTape.mode &&
    (diagnosticComputedTape.mode === 'record' ||
      diagnosticComputedTape.layer === 'both' ||
      diagnosticComputedTape.layer === layer)
  );
};
const diagnosticComputedReplay = (layer, element, name, compute) => {
  diagnosticComputedTapeStats ||= globalThis.__mimicComputedTapeStats = {};
  const stats = (diagnosticComputedTapeStats[layer] ||= {
    hits: 0,
    misses: 0,
    mismatches: 0,
    replayed: 0,
  });
  const key = JSON.stringify([layer, elementSlot(element).nodeId, name]);
  const encoded = diagnosticComputedTape.rows?.[key];
  stats[encoded != null ? 'hits' : 'misses']++;
  if (diagnosticComputedTape.mode === 'replay' && encoded != null) {
    stats.replayed++;
    diagnosticComputedTapeDecoded ||= new Map();
    if (diagnosticComputedTapeDecoded.has(key)) return diagnosticComputedTapeDecoded.get(key);
    const [type, value] = JSON.parse(encoded);
    const result = type === 'undefined' ? undefined : value;
    diagnosticComputedTapeDecoded.set(key, result);
    return result;
  }
  const result = compute();
  const actual = JSON.stringify([typeof result, result]);
  if (diagnosticComputedTape.mode === 'record') host.diagnosticComputedTape('record', key, actual);
  else if (encoded != null && actual !== encoded) {
    stats.mismatches++;
    host.diagnosticComputedTape('MISMATCH', key, actual);
  }
  return result;
};
const diagnosticResolveCSSComputedValue = (element, name) => {
  if (!diagnosticComputedLayerEnabled('computed')) return resolveCSSComputedValue(element, name);
  return diagnosticComputedReplay('computed', element, name, () =>
    resolveCSSComputedValue(element, name),
  );
};
