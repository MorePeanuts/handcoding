const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const root = path.resolve(__dirname, '../..');
function run(t, layer, packages, extra = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'hc-test-layers-'));
  t.after(() => fs.rmSync(dir, {recursive: true, force: true}));
  fs.writeFileSync(path.join(dir, 'go'), `#!/bin/sh
if [ "$1" = list ]; then printf '%s\\n' "$FAKE_PACKAGES"; exit 0; fi
printf '%s\\t' "$@" >> "$FAKE_LOG"
printf '\\n' >> "$FAKE_LOG"
exit "\${FAKE_EXIT:-0}"
`, {mode: 0o755});
  const result = spawnSync('bash', ['scripts/test.sh', layer, '-count=1', '-parallel=4', '-json'], {
    cwd: root, encoding: 'utf8', env: {...process.env, PATH: dir + path.delimiter + process.env.PATH,
      TEST_PACKAGES: '', TEST_PACKAGE_JOBS: '', TEST_PACKAGE_PARALLEL: '',
      FAKE_PACKAGES: packages.join('\n'), FAKE_LOG: path.join(dir, 'calls'), ...extra}});
  const calls = fs.existsSync(path.join(dir, 'calls')) ? fs.readFileSync(path.join(dir, 'calls'), 'utf8').trim().split('\n').map(l => l.trim().split('\t')) : [];
  const events = result.stdout.trim().split('\n').map(l => JSON.parse(l));
  return {result, calls, events};
}
const prefix = 'example/internal/';
test('integration isolates app then runs every other selected package once', t => {
  const {result, calls, events} = run(t, 'integration', ['tui', 'app', 'lang/c', 'workspace'].map(p => prefix + p), {TEST_PACKAGE_JOBS: '2', TEST_PACKAGE_PARALLEL: '2'});
  assert.equal(result.status, 0, result.stderr); assert.equal(calls.length, 2);
  assert.deepEqual(calls[0].filter(a => a.startsWith(prefix)), [prefix + 'app']);
  assert.deepEqual(calls[1].filter(a => a.startsWith(prefix)), [prefix + 'tui', prefix + 'lang/c']);
  assert.ok(calls[0].includes('-p=1')); assert.ok(calls[1].includes('-p=2')); assert.ok(calls[1].includes('-parallel=2'));
  for (const args of calls) for (const flag of ['-race', '-count=1', '-skip']) assert.ok(args.includes(flag));
  assert.deepEqual(events.map(e => e.HCPhase), ['start', 'end']);
});
test('frontend-only selection does not introduce app or omit the selected package', t => {
  const {result, calls} = run(t, 'integration', [prefix + 'tui'], {TEST_PACKAGE_JOBS: '2'});
  assert.equal(result.status, 0); assert.equal(calls.length, 1);
  assert.deepEqual(calls[0].filter(a => a.startsWith(prefix)), [prefix + 'tui']);
});
test('new packages default to base and regression remains the complete corpus selector', t => {
  const unit = run(t, 'unit', [prefix + 'new', prefix + 'app']);
  assert.deepEqual(unit.calls[0].filter(a => a.startsWith(prefix)), [prefix + 'new']);
  const regression = run(t, 'regression', []);
  assert.ok(regression.calls[0].includes('-run'));
  assert.equal(regression.calls[0].at(-1), './internal/app');
  for (const name of ['TestRegressionReferences', 'TestGoMinimumVersion', 'TestCPPMinimumVersion', 'TestCMinimumVersion', 'TestECMAScriptReferences']) assert.ok(regression.calls[0].some(a => a.includes(name)));
});
test('failure exit codes reach the layer marker and invalid concurrency fails', t => {
  const failure = run(t, 'integration', [prefix + 'app'], {TEST_PACKAGE_JOBS: '2', FAKE_EXIT: '7'});
  assert.equal(failure.result.status, 7); assert.equal(failure.events.at(-1).Status, 7);
  const invalid = run(t, 'integration', [prefix + 'tui'], {TEST_PACKAGE_JOBS: '0'});
  assert.equal(invalid.result.status, 1); assert.equal(invalid.calls.length, 0);
});
