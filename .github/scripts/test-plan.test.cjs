const {test} = require('node:test');
const assert = require('node:assert/strict');
const {plan, LANGUAGES, SMOKE} = require('./test-plan.cjs');
const knownProblems = [...SMOKE, 'reverse-linked-list'];
const select = (files, extra = {}) => plan(files, {knownProblems, ...extra});

test('prose skips expensive jobs, embedded content does not', () => {
  const docs = select(['README.md', 'docs/how-to-use_zh.md', 'testdata/README.md', 'testdata/README_zh.md',
    'testdata/leetcode-regression/.gitignore', 'testdata/protocol-regression/.gitignore']);
  assert.equal(docs.unit, false); assert.deepEqual(docs.languages, []);
  for (const file of ['docs/protocol.md', 'internal/builtin/foo/SKILL.md', 'new-package/README.md', 'testdata/unknown/README.md', 'testdata/leetcode-regression/problems/new/.gitignore']) {
    const p = select([file]); assert.equal(p.unit, true); assert.ok(p.integration.length);
  }
});
test('frontend changes exercise their integration without a corpus', () => {
  const p = select(['internal/tui/packs.go']);
  assert.equal(p.unit, true); assert.equal(p.lint, true);
  assert.deepEqual(p.integration, ['./internal/tui']); assert.deepEqual(p.languages, []);
});
test('driver PR smoke becomes full affected-language regression on main', () => {
  const files = ['internal/lang/c/runtime.c.txt'];
  const pr = select(files), main = select(files, {event: 'push'});
  assert.deepEqual(pr.languages, ['c']); assert.deepEqual(pr.selection.c, [...SMOKE].sort());
  assert.deepEqual(main.languages, ['c']); assert.deepEqual(main.selection.c, []);
  assert.deepEqual(select(['internal/lang/javascript/runtime.js']).languages, ['javascript', 'typescript']);
  // Python also implements generators and checkers for every language.
  assert.deepEqual(select(['internal/lang/python/hc_checker.py']).languages, LANGUAGES);
});
test('changed reference selects its language and problem; shared cases select all languages', () => {
  const path = 'testdata/leetcode-regression/problems/0206-reverse-linked-list/';
  for (const [file, language] of [['solution.c', 'c'], ['Solution.java', 'java'], ['solution.ts', 'typescript']]) {
    const p = select([path + 'reference/' + file]);
    assert.deepEqual(p.languages, [language]); assert.deepEqual(p.selection[language], ['reverse-linked-list']);
  }
  assert.deepEqual(select([path + 'cases.jsonl']).languages, LANGUAGES);
  const mixed = select([path + 'reference/solution.c', 'internal/lang/java/java.go']);
  assert.deepEqual(mixed.languages, ['c', 'java']); assert.ok(mixed.selection.c.includes('reverse-linked-list'));
});
test('shared, unknown, deleted problems and unavailable diffs fail toward full coverage', () => {
  for (const files of [['internal/runner/run.go'], ['go.mod'], ['.github/workflows/ci.yml'],
    ['scripts/test.sh'], ['new-file'], ['testdata/leetcode-regression/problems/removed/cases.jsonl'], null]) {
    assert.deepEqual(select(files).languages, LANGUAGES);
  }
  assert.equal(select(null).mode, 'full');
  assert.deepEqual(select(['README.md'], {full: true}).languages, LANGUAGES);
  assert.deepEqual(select(['README.md'], {full: true}).selection.java, []);
  assert.equal(select(['new-file']).mode, 'full');
});
test('large diffs and both sides of renames retain coverage', () => {
  const files = Array.from({length: 400}, () => 'README.md');
  files.push('internal/lang/c/old.c', 'internal/lang/java/new.java');
  assert.deepEqual(select(files).languages, ['c', 'java']);
});

test('a full C corpus does not expand every other language in a mixed PR', () => {
  const p = select(['internal/app/app.go', 'testdata/leetcode-regression/pack.yaml',
    'testdata/leetcode-regression/problems/0206-reverse-linked-list/reference/solution.c']);
  assert.ok(p.selection.c.includes('reverse-linked-list'));
  assert.deepEqual(p.selection.java, [...SMOKE].sort());
  assert.deepEqual(select(['internal/lang/c/c.go'], {event: 'push'}).selection, {c: []});
});

test('protocol extension pack changes retain targeted regression coverage', () => {
  const p = select(['testdata/protocol-regression/problems/prefix-sums-3d/reference/solution.c']);
  assert.deepEqual(p.selection, {c: ['prefix-sums-3d']});
  const cases = select(['testdata/protocol-regression/problems/prefix-sums-3d/cases.jsonl']);
  assert.deepEqual(cases.languages, LANGUAGES);
  assert.deepEqual(cases.selection.go, ['prefix-sums-3d']);
  assert.deepEqual(select(['testdata/protocol-regression/pack.yaml']).languages, LANGUAGES);
});
