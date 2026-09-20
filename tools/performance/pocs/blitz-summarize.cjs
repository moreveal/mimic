// Read-only summary of the unchanged Wikipedia matrix and optional diagnostic
// server log. Usage: node blitz-summarize.cjs MATRIX_DIR_OR_JSON [PROFILE_LOG]
// Timings are wall milliseconds, not CPU. Memory is post-Page-close process
// memory in MiB (not a measurement of retained native state alone).
const fs = require('node:fs');
const path = require('node:path');

function statistics(values) {
  const sorted = values.filter(Number.isFinite).sort((a, b) => a - b);
  if (!sorted.length) return { count: 0, median: null, min: null, max: null, sum: 0 };
  const middle = Math.floor(sorted.length / 2);
  return {
    count: sorted.length,
    median: sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2,
    min: sorted[0],
    max: sorted.at(-1),
    sum: sorted.reduce((sum, value) => sum + value, 0),
  };
}

function fields(records, select) {
  const names = new Set(records.flatMap((record) => Object.keys(select(record) || {})));
  return Object.fromEntries(
    [...names]
      .sort()
      .map((name) => [name, statistics(records.map((record) => select(record)?.[name]))]),
  );
}

function summarizeMatrix(records) {
  if (!Array.isArray(records)) throw new Error('Expected results.json array');
  const seen = new Set();
  for (const row of records) {
    const key = JSON.stringify([row.iteration, row.mode, row.phase]);
    if (seen.has(key)) throw new Error(`Duplicate matrix sample ${key}`);
    seen.add(key);
    if (!Number.isFinite(row.wallMS)) throw new Error(`Invalid wallMS in ${key}`);
  }
  const modes = [...new Set(records.map((row) => row.mode))];
  const phases = [...new Set(records.map((row) => row.phase))];
  const groups = {};
  for (const mode of modes) {
    groups[mode] = {};
    for (const phase of phases) {
      const rows = records.filter((row) => row.mode === mode && row.phase === phase);
      groups[mode][phase] = {
        wallMS: statistics(rows.map((row) => row.wallMS)),
        stagesMS: fields(rows, (row) => row.stages),
        // The harness wall envelope includes work not assigned to a named stage.
        unassignedEnvelopeMS: statistics(
          rows.map(
            (row) => row.wallMS - Object.values(row.stages || {}).reduce((a, b) => a + b, 0),
          ),
        ),
        postPageCloseMiB: fields(rows, (row) =>
          Object.fromEntries(
            Object.entries(row.afterPageClose || {}).map(([key, bytes]) => [key, bytes / 2 ** 20]),
          ),
        ),
      };
    }
  }
  const comparisons = {};
  for (const candidate of modes.filter((mode) => mode !== 'baseline')) {
    comparisons[candidate] = {};
    for (const phase of phases) {
      const baseline = records.filter((row) => row.mode === 'baseline' && row.phase === phase);
      const candidates = records.filter((row) => row.mode === candidate && row.phase === phase);
      const paired = baseline.flatMap((control) => {
        const treatment = candidates.find((row) => row.iteration === control.iteration);
        if (!treatment) return [];
        return [
          {
            iteration: control.iteration,
            deltaMS: treatment.wallMS - control.wallMS,
            deltaPercent: 100 * (treatment.wallMS / control.wallMS - 1),
            stages: Object.fromEntries(
              Object.keys(control.stages || {})
                .filter((name) => Number.isFinite(treatment.stages?.[name]))
                .map((name) => [name, treatment.stages[name] - control.stages[name]]),
            ),
          },
        ];
      });
      comparisons[candidate][phase] = {
        baselineCount: baseline.length,
        candidateCount: candidates.length,
        pairedCount: paired.length,
        unpairedBaseline: baseline
          .filter((row) => !paired.some((pair) => pair.iteration === row.iteration))
          .map((row) => row.iteration),
        unpairedCandidate: candidates
          .filter((row) => !paired.some((pair) => pair.iteration === row.iteration))
          .map((row) => row.iteration),
        deltaMS: statistics(paired.map((pair) => pair.deltaMS)),
        deltaPercent: statistics(paired.map((pair) => pair.deltaPercent)),
        stageDeltaMS: fields(paired, (pair) => pair.stages),
        pairs: paired,
      };
    }
  }
  return {
    groups,
    comparisons,
    deltaConvention: 'candidate minus baseline; negative means faster',
  };
}

function summarizeProfile(text) {
  const accounting = [],
    native = [],
    calls = {},
    legacy = [];
  let snapshotsBuilt = 0,
    snapshotPeakBytes = 0;
  for (const [lineIndex, line] of text.split(/\r?\n/).entries()) {
    const match = line.match(/BLITZ (accounting|native-phases|calls|legacy) (.*)/);
    if (match) {
      const at = match[2].indexOf('{');
      if (at < 0 && match[1] === 'legacy') {
        legacy.push(match[2]);
        continue;
      }
      if (at < 0) throw new Error(`Missing profile JSON at line ${lineIndex + 1}`);
      let data;
      try {
        data = JSON.parse(match[2].slice(at));
      } catch (error) {
        throw new Error(`Malformed profile JSON at line ${lineIndex + 1}: ${error.message}`);
      }
      if (match[1] === 'accounting') accounting.push(data);
      if (match[1] === 'native-phases') native.push(data);
      if (match[1] === 'legacy') legacy.push(data);
      if (match[1] === 'calls') {
        // reportBlitzCalls clears counters after emitting; records are additive.
        for (const [name, value] of Object.entries(data)) {
          calls[name] ||= { count: 0, wallMS: 0 };
          calls[name].count += value.Count;
          calls[name].wallMS += value.Micros / 1000;
        }
      }
    }
    const snapshot = line.match(/BLITZ snapshot .*built=(true|false).*bytes=(\d+)/);
    if (snapshot) {
      snapshotsBuilt += Number(snapshot[1] === 'true');
      snapshotPeakBytes = Math.max(snapshotPeakBytes, Number(snapshot[2]));
    }
  }
  const outcomes = [
    ...new Set([
      'first-build',
      'owner-rebuild',
      'rebuild',
      'reuse',
      'fallback',
      'error',
      ...accounting.map((row) => row.outcome),
    ]),
  ];
  return {
    accountingByOutcome: Object.fromEntries(
      outcomes.map((outcome) => {
        const rows = accounting.filter((row) => row.outcome === outcome);
        return [
          outcome,
          {
            totalWallMS: statistics(rows.map((row) => row.totalWallMS)),
            phasesWallMS: fields(rows, (row) => row.phasesWallMS),
            reasons: [...new Set(rows.map((row) => row.fallback || row.error).filter(Boolean))],
          },
        ];
      }),
    ),
    producerEnvelopeWallMS: statistics(accounting.map((row) => row.totalWallMS)),
    // Nested inside nativeResolve, never added to the producer envelope.
    // Keep documents separate: a parent subdocuments phase may enclose child
    // phase records. Summing their inclusive totals would double-count work.
    nativeByDocument: Object.fromEntries(
      [...new Set(native.map((row) => row.document))].map((document) => {
        const rows = native.filter((row) => row.document === document);
        return [
          document,
          { records: rows.length, phasesWallMS: fields(rows, (row) => row.phasesWallMS) },
        ];
      }),
    ),
    calls,
    legacyRecords: legacy,
    snapshots: { builtRecords: snapshotsBuilt, largestPacketBytes: snapshotPeakBytes },
    caveats: [
      'Profile is a separate instrumented run, not timing evidence for the uninstrumented matrix.',
      'Calls, snapshots, sync, native phases and accounting envelopes overlap; do not sum them.',
      'Missing reuse/fallback records are not proof that no such calls occurred without profile coverage.',
      'Document identifiers are scoped to a process; pass one server log, not concatenated processes.',
    ],
  };
}

if (require.main === module) {
  const [matrixInput, profileInput] = process.argv.slice(2);
  if (!matrixInput)
    throw new Error('Usage: node blitz-summarize.cjs MATRIX_DIR_OR_JSON [PROFILE_LOG]');
  const input = fs.statSync(matrixInput).isDirectory()
    ? path.join(matrixInput, 'results.json')
    : matrixInput;
  const output = { matrix: summarizeMatrix(JSON.parse(fs.readFileSync(input, 'utf8'))) };
  if (profileInput) output.profile = summarizeProfile(fs.readFileSync(profileInput, 'utf8'));
  process.stdout.write(JSON.stringify(output, null, 2) + '\n');
}

module.exports = { statistics, summarizeMatrix, summarizeProfile };
