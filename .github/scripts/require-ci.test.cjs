const assert = require('node:assert/strict');
const { test } = require('node:test');
const requireCI = require('./require-ci.cjs');

const sha = 'a'.repeat(40);
const passed = {
  id: 1, head_sha: sha, head_branch: 'main', event: 'push',
  head_repository: { full_name: 'owner/repo' },
  status: 'completed', conclusion: 'success', html_url: 'https://example.test/run/1',
};
function check(runs) {
  const listWorkflowRuns = Symbol('workflow endpoint');
  const github = {
    rest: { actions: { listWorkflowRuns } },
    paginate: async (endpoint, options) => {
      assert.equal(endpoint, listWorkflowRuns);
      assert.deepEqual(options, {
        owner: 'owner', repo: 'repo', workflow_id: 'ci.yml',
        head_sha: sha, event: 'push', per_page: 100,
      });
      return runs;
    },
  };
  return requireCI({ github, owner: 'owner', repo: 'repo', sha });
}

test('accepts completed CI on the exact main or maintenance commit', async () => {
  assert.equal(await check([passed]), passed);
  const maintenance = { ...passed, head_branch: 'v0.1.x' };
  assert.equal(await check([maintenance]), maintenance);
});

test('does not substitute a PR, another commit, fork, or feature branch', async () => {
  for (const change of [
    { head_sha: 'b'.repeat(40) }, { event: 'pull_request' },
    { head_repository: { full_name: 'fork/repo' } }, { head_repository: null },
    { head_branch: 'fix/general' },
  ]) await assert.rejects(check([{ ...passed, ...change }]), /No main\/maintenance push CI/);
  await assert.rejects(check([]), /No main\/maintenance push CI/);
});

test('latest run must pass; stale successes cannot bypass pending or failed CI', async () => {
  for (const change of [
    { status: 'queued', conclusion: null }, { status: 'in_progress', conclusion: null },
    { conclusion: 'failure' }, { conclusion: 'cancelled' }, { conclusion: 'timed_out' },
    { conclusion: 'skipped' }, { conclusion: 'neutral' },
  ]) {
    const latest = { ...passed, id: 2, ...change };
    await assert.rejects(check([passed, latest]), /CI has not passed/);
    await assert.rejects(check([latest, passed]), /CI has not passed/);
  }
});

test('accepts the latest successful rerun after a failure', async () => {
  const latest = { ...passed, id: 2 };
  assert.equal(await check([latest, { ...passed, conclusion: 'failure' }]), latest);
});

test('API failures and unresolved tags fail closed', async () => {
  const github = {
    rest: { actions: { listWorkflowRuns: null } },
    paginate: async () => { throw new Error('API unavailable'); },
  };
  await assert.rejects(requireCI({ github, owner: 'owner', repo: 'repo', sha }), /API unavailable/);
  await assert.rejects(requireCI({ github, owner: 'owner', repo: 'repo', sha: 'v0.1.0' }), /full commit SHA/);
});
