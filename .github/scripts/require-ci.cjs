// Only CI for the actual release commit counts. A PR's tested merge commit
// is not interchangeable with the squash commit subsequently tagged on main.
module.exports = async function requireCI({ github, owner, repo, sha }) {
  if (!/^[0-9a-f]{40}$/.test(sha)) throw new Error('Expected a full commit SHA');
  const runs = await github.paginate(github.rest.actions.listWorkflowRuns, {
    owner, repo, workflow_id: 'ci.yml', head_sha: sha, event: 'push', per_page: 100,
  });
  const eligible = runs.filter(run =>
    run.head_sha === sha && run.event === 'push' &&
    run.head_repository?.full_name === `${owner}/${repo}` &&
    (run.head_branch === 'main' || /^v\d+(?:\.\d+)*\.x$/.test(run.head_branch))
  );
  // Do not select an older green run while a newer run is pending or failed.
  eligible.sort((a, b) => b.id - a.id);
  const latest = eligible[0];
  if (!latest) {
    throw new Error(`No main/maintenance push CI found for ${sha}; publish only after CI passes for this exact commit`);
  }
  if (latest.status !== 'completed' || latest.conclusion !== 'success') {
    throw new Error(`CI has not passed for ${sha}: ${latest.status}/${latest.conclusion ?? 'pending'} (${latest.html_url}). Rerun Release after this CI succeeds`);
  }
  return latest;
};
