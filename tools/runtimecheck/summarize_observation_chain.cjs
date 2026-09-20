const fs = require('node:fs');
const data = JSON.parse(
  fs.readFileSync(process.argv[2] || '.build/observation-chain-results.json'),
);
const median = (values) => {
  const sorted = values.filter(Number.isFinite).sort((a, b) => a - b);
  if (!sorted.length) return null;
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2;
};
const rounded = (value) => (value === null ? null : Math.round(value * 100) / 100);
const results = [];
for (const browser of ['mimic', 'chrome']) {
  for (const n of data.counts) {
    const rows = data.rows.filter((row) => row.ok && row.browser === browser && row.n === n);
    if (!rows.length) continue;
    const names = [...new Set(rows.flatMap((row) => row.stages.map((stage) => stage.name)))];
    const stages = Object.fromEntries(
      names.map((name) => {
        const matches = rows.flatMap((row) => row.stages.filter((stage) => stage.name === name));
        return [
          name,
          {
            samples: matches.length,
            wallMs: rounded(median(matches.map((stage) => stage.wallMs))),
            rectMs: rounded(median(matches.map((stage) => stage.result?.rectMs))),
            styleMs: rounded(median(matches.map((stage) => stage.result?.styleMs))),
            pageMs: rounded(median(matches.map((stage) => stage.result?.pageMs))),
          },
        ];
      }),
    );
    results.push({
      browser,
      n,
      rows: rows.length,
      totalMs: rounded(median(rows.map((row) => row.totalMs))),
      buildMs: rounded(median(rows.map((row) => row.buildMs))),
      firstDirectByOrder: Object.fromEntries(
        ['rect', 'style'].map((order) => [
          order,
          rounded(
            median(
              rows
                .filter((row) => row.order === order)
                .map(
                  (row) =>
                    row.stages.find((stage) => stage.name === 'fixture.build-and-first-direct')
                      ?.result.firstDirectMs,
                ),
            ),
          ),
        ]),
      ),
      stages,
    });
  }
}
console.log(
  JSON.stringify(
    {
      finished: data.finishedAt || null,
      failures: data.rows
        .filter((row) => row.ok === false)
        .map(({ browser, n, order, error }) => ({ browser, n, order, error })),
      results,
    },
    null,
    2,
  ),
);
