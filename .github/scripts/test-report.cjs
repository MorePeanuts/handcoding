const fs = require('node:fs');
const path = require('node:path');

function unionSeconds(intervals) {
  let total = 0, end = -Infinity;
  for (const [start, stop] of intervals.sort((a, b) => a[0] - b[0])) {
    total += Math.max(0, stop - Math.max(start, end));
    end = Math.max(end, stop);
  }
  return total / 1000;
}

function analyze(name, text) {
  const packages = new Map(), groups = new Map();
  let start, end, status;
  for (const line of text.split('\n')) {
    let event;
    try { event = JSON.parse(line); } catch { continue; }
    const time = Date.parse(event.Time);
    if (event.HCPhase === 'start') { start = time; continue; }
    if (event.HCPhase === 'end') { end = time; status = event.Status; continue; }
    if (!event.Package) continue;
    if (!event.Test) {
      const pkg = packages.get(event.Package) || {name: event.Package};
      if (event.Action === 'start') pkg.start = time;
      if (['pass', 'fail', 'skip'].includes(event.Action)) {
        pkg.end = time; pkg.elapsed = event.Elapsed || 0; pkg.result = event.Action;
      }
      packages.set(event.Package, pkg);
      continue;
    }
    const root = event.Test.split('/')[0], key = `${event.Package}/${root}`;
    const group = groups.get(key) || {name: key, nodes: new Map()};
    const node = group.nodes.get(event.Test) || {};
    if (event.Action === 'run') node.start = time;
    if (event.Action === 'cont') node.cont = time;
    if (['pass', 'fail', 'skip'].includes(event.Action)) {
      node.end = time; node.elapsed = event.Elapsed || 0; node.result = event.Action;
    }
    group.nodes.set(event.Test, node);
    groups.set(key, group);
  }
  const rows = [...packages.values()];
  const intervals = rows.filter(p => Number.isFinite(p.start) && Number.isFinite(p.end)).map(p => [p.start, p.end]);
  const tests = [...groups.values()].map(group => {
    const root = group.nodes.get(group.name.slice(group.name.lastIndexOf('/') + 1)) || {};
    const parents = new Set();
    for (const name of group.nodes.keys()) {
      let parent = name;
      while (parent.includes('/')) { parent = parent.slice(0, parent.lastIndexOf('/')); parents.add(parent); }
    }
    const leaves = [...group.nodes].filter(([name, node]) => !parents.has(name) && node.result);
    return {name: group.name, result: root.result || 'incomplete',
      span: Number.isFinite(root.end) ? (root.end - (root.cont ?? root.start)) / 1000 : null,
      leaves: leaves.length, skipped: leaves.filter(([, n]) => n.result === 'skip').length,
      leafSeconds: leaves.reduce((sum, [, n]) => sum + n.elapsed, 0)};
  });
  const wall = Number.isFinite(start) && Number.isFinite(end) ? (end - start) / 1000 : null;
  const running = unionSeconds(intervals);
  return {name: path.basename(name).replace(/^test-/, '').replace(/\.jsonl$/, ''), wall, running,
    outside: wall === null ? null : Math.max(0, wall - running), status, packages: rows, tests};
}

const seconds = value => value == null || !Number.isFinite(value) ? '—' : value.toFixed(2);
const short = value => value.replace('github.com/MorePeanuts/handcoding/', '');
function render(layers) {
  let report = '## Test timings\n\nTimes are wall-clock seconds, not CPU usage. Layer wall time includes build/startup and test execution; package intervals are unioned to avoid double-counting concurrent packages.\n\n';
  report += '| Layer | Wall | Test binaries active | Outside test binaries | Exit |\n|---|---:|---:|---:|---|\n';
  for (const l of layers) report += `| ${l.name} | ${seconds(l.wall)} | ${seconds(l.running)} | ${seconds(l.outside)} | ${l.status ?? 'incomplete'} |\n`;
  report += '\nOutside-test time includes compilation, vet, discovery and process startup; it is not a CPU profile. Missing end markers are reported as incomplete.\n\n| Layer / package | Seconds | Result |\n|---|---:|---|\n';
  for (const l of layers) for (const p of l.packages) report += `| ${l.name} / ${short(p.name)} | ${seconds(p.elapsed)} | ${p.result || 'incomplete'} |\n`;
  report += '\n### Slowest test groups\n\nSpan runs from CONT (or RUN for serial tests) to completion, including parallel child waits. Spans can overlap. Leaf seconds sum only terminal leaf tests and are also not additive wall time. Parent Elapsed alone omits parallel child waits.\n\n| Layer / group | Span | Leaf tests | Skipped | Leaf seconds | Result |\n|---|---:|---:|---:|---:|---|\n';
  const tests = layers.flatMap(l => l.tests.map(t => ({...t, layer: l.name})));
  for (const t of tests.sort((a, b) => (b.span ?? 0) - (a.span ?? 0)).slice(0, 20)) {
    report += `| ${t.layer} / ${short(t.name)} | ${seconds(t.span)} | ${t.leaves} | ${t.skipped} | ${seconds(t.leafSeconds)} | ${t.result} |\n`;
  }
  return report;
}

module.exports = {analyze, render};
if (require.main === module) {
  const layers = process.argv.slice(2).filter(file => fs.existsSync(file)).map(file => analyze(file, fs.readFileSync(file, 'utf8')));
  fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, render(layers));
}
