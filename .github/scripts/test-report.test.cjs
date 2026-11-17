const {test} = require('node:test');
const assert = require('node:assert/strict');
const {analyze, render} = require('./test-report.cjs');
const event = (seconds, fields) => JSON.stringify({Time: new Date(seconds * 1000).toISOString(), ...fields});

test('parallel children use actual parent span, not its short Elapsed', () => {
  const log = [event(0, {HCPhase: 'start'}), event(2, {Action: 'start', Package: 'p'}),
    event(2, {Action: 'run', Package: 'p', Test: 'TestCorpus'}),
    event(4, {Action: 'cont', Package: 'p', Test: 'TestCorpus'}),
    event(5, {Action: 'pass', Package: 'p', Test: 'TestCorpus/pack/a', Elapsed: 1}),
    event(10, {Action: 'pass', Package: 'p', Test: 'TestCorpus/pack/b', Elapsed: 5}),
    event(10, {Action: 'pass', Package: 'p', Test: 'TestCorpus', Elapsed: 0.1}),
    event(11, {Action: 'pass', Package: 'p', Elapsed: 9}), event(12, {HCPhase: 'end', Status: 0})].join('\n');
  const result = analyze('test-regression.jsonl', log);
  assert.equal(result.wall, 12); assert.equal(result.outside, 3);
  assert.equal(result.tests[0].span, 6); assert.equal(result.tests[0].leaves, 2);
  assert.equal(result.tests[0].leafSeconds, 6);
  assert.match(render([result]), /regression \/ p\/TestCorpus \| 6.00 \| 2/);
});

test('overlapping packages are unioned and build gaps remain visible', () => {
  const log = [event(0, {HCPhase: 'start'}), event(2, {Action: 'start', Package: 'a'}),
    event(4, {Action: 'start', Package: 'b'}), event(6, {Action: 'pass', Package: 'a', Elapsed: 4}),
    event(8, {Action: 'pass', Package: 'b', Elapsed: 4}), event(10, {HCPhase: 'end', Status: 0})].join('\n');
  const result = analyze('test-integration.jsonl', log);
  assert.equal(result.running, 6); assert.equal(result.outside, 4);
});

test('failure, skips, malformed lines and interrupted logs are explicit', () => {
  const log = ['not JSON', event(0, {HCPhase: 'start'}), event(1, {Action: 'start', Package: 'p'}),
    event(1, {Action: 'run', Package: 'p', Test: 'TestParent'}),
    event(2, {Action: 'skip', Package: 'p', Test: 'TestParent/skip'}),
    event(3, {Action: 'fail', Package: 'p', Test: 'TestParent/fail', Elapsed: 1}),
    event(3, {Action: 'fail', Package: 'p', Test: 'TestParent', Elapsed: 2}),
    event(4, {Action: 'fail', Package: 'p', Elapsed: 3})].join('\n');
  const incomplete = analyze('test-unit.jsonl', log);
  assert.equal(incomplete.wall, null); assert.match(render([incomplete]), /incomplete/);
  assert.equal(incomplete.tests[0].skipped, 1); assert.equal(incomplete.tests[0].leafSeconds, 1);
  const failed = analyze('test-unit.jsonl', log + '\n' + event(5, {HCPhase: 'end', Status: 1}));
  assert.equal(failed.status, 1); assert.equal(failed.tests[0].result, 'fail');
});
