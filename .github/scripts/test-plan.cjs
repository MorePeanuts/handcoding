// Keep the rules conservative: unknown paths, shared execution code and
// dependency changes select every language. No GitHub API pagination/diff limit.
const LANGUAGES = ['go', 'python', 'c', 'cpp', 'java', 'javascript', 'typescript'];
const INTEGRATION = ['./internal/app', './internal/cli', './internal/tui', './internal/runner', './internal/leetcode', './internal/lang/...'];
const SMOKE = ['two-sum', 'longest-palindromic-substring', 'remove-element', 'lru-cache',
  'serialize-and-deserialize-binary-tree', 'copy-list-with-random-pointer',
  'intersection-of-two-linked-lists', 'prefix-sums-3d'];
const extensions = {go: 'go', py: 'python', c: 'c', cpp: 'cpp', java: 'java', js: 'javascript', ts: 'typescript'};

function plan(files, {event = 'pull_request', full = false, knownProblems = []} = {}) {
  const packages = new Set(), languages = new Set(), reasons = new Set();
  const broadLanguages = new Set(), targets = new Map();
  let unit = false, lint = false, fallback = false;
  const addPackages = (...values) => values.forEach(p => packages.add(p));
  const addLanguages = values => values.forEach(l => languages.add(l));
  const all = (reason, force = false) => {
    unit = lint = true; fallback ||= force;
    addPackages(...INTEGRATION); addLanguages(LANGUAGES);
    LANGUAGES.forEach(l => broadLanguages.add(l)); reasons.add(reason);
  };
  if (full || files === null) all(full ? 'Explicit full validation' : 'Diff unavailable: full validation');
  else for (const file of files) {
    // Only known prose is exempt. Embedded docs/skills and unknown Markdown
    // are code inputs, so a blanket *.md exclusion would be unsafe.
    if (/^(README(?:_zh)?\.md|CONTRIBUTING\.md|CLAUDE\.md|LICENSE)$/.test(file) ||
        /^docs\/(how-to-use(?:_zh)?|packs(?:_zh)?|protocol_zh)\.md$/.test(file)) {
      reasons.add('Documentation only'); continue;
    }
    unit = lint = true;
    const driver = file.match(/^internal\/lang\/(golang|python|c|cpp|java|javascript)\//);
    if (driver && driver[1] !== 'python') {
      const id = driver[1] === 'golang' ? 'go' : driver[1];
      const affected = id === 'javascript' ? ['javascript', 'typescript'] : [id];
      addLanguages(affected); affected.forEach(l => broadLanguages.add(l));
      addPackages(`./internal/lang/${driver[1]}`, './internal/app', './internal/cli', './internal/tui', './internal/leetcode');
      reasons.add(`Driver: ${id}`); continue;
    }
    const corpus = file.match(/^testdata\/(?:leetcode-regression|protocol-regression)\/problems\/([^/]+)\/(.+)$/);
    if (corpus) {
      const slug = corpus[1].replace(/^\d+-/, '');
      if (!knownProblems.includes(slug)) { all('Removed or unrecognized regression problem', true); continue; }
      const reference = corpus[2].match(/^reference\/(?:solution\.(go|py|c|cpp|js|ts)|Solution\.(java))$/);
      const affected = reference ? [extensions[reference[1] || reference[2]]] : LANGUAGES;
      addLanguages(affected);
      for (const language of affected) {
        if (!targets.has(language)) targets.set(language, new Set());
        targets.get(language).add(slug);
      }
      reasons.add(`Regression problem: ${slug}`); continue;
    }
    const frontend = file.match(/^internal\/(tui|cli|ui|markdown|termimg)\//);
    if (frontend) {
      addPackages(frontend[1] === 'cli' ? './internal/cli' : './internal/tui');
      if (frontend[1] === 'markdown') addPackages('./internal/cli');
      reasons.add(`Frontend: ${frontend[1]}`); continue;
    }
    if (/^internal\/(builtin|skill|agent)\//.test(file)) {
      addPackages('./internal/app', './internal/cli', './internal/tui');
      reasons.add('Agent/skill integration'); continue;
    }
    const shared = /^(testdata\/(?:leetcode-regression|protocol-regression)\/pack\.yaml$|internal\/(app|runner|judge|problem|workspace|pack|git|store|regression|testenv|version|leetcode)\/|internal\/lang\/|cmd\/|docs\/docs\.go$|docs\/protocol\.md$|\.github\/|scripts\/|go\.(mod|sum)$|Makefile$|pyproject\.toml$|uv\.lock$|\.python-version$|\.golangci|\.goreleaser)/.test(file);
    all(`${shared ? 'Shared' : 'Unclassified'}: ${file}`, !shared);
  }
  const mode = full || fallback || files === null || event !== 'pull_request' ? 'full' : 'smoke';
  const selection = Object.fromEntries(LANGUAGES.filter(l => languages.has(l)).map(l => {
    const selected = broadLanguages.has(l)
      ? (mode === 'full' ? [] : [...new Set([...SMOKE, ...(targets.get(l) || [])])])
      : [...(targets.get(l) || [])];
    return [l, selected.sort()];
  }));
  return {unit, lint, selection, integration: [...packages].sort(), languages: LANGUAGES.filter(l => languages.has(l)),
    mode, reasons: [...reasons].sort()};
}

module.exports = {plan, LANGUAGES, INTEGRATION, SMOKE};

if (require.main === module) {
  const fs = require('node:fs');
  const {execFileSync} = require('node:child_process');
  const event = JSON.parse(fs.readFileSync(process.env.GITHUB_EVENT_PATH, 'utf8'));
  const kind = process.env.GITHUB_EVENT_NAME;
  let files = null;
  try {
    let base = kind === 'pull_request' ? event.pull_request.base.sha : event.before;
    const head = kind === 'pull_request' ? event.pull_request.head.sha : process.env.GITHUB_SHA;
    if (!/^[a-f0-9]{40}$/.test(base) || /^0+$/.test(base)) throw new Error('No diff base');
    if (kind === 'pull_request') base = execFileSync('git', ['merge-base', base, head], {encoding: 'utf8'}).trim();
    // --no-renames includes both sides of moves so old coverage isn't lost.
    files = execFileSync('git', ['diff', '--no-renames', '--name-only', '-z', base, head], {maxBuffer: 16 * 1024 * 1024})
      .toString().split('\0').filter(Boolean);
  } catch (error) { console.log(`Full fallback: ${error.message}`); }
  const knownProblems = ['leetcode-regression', 'protocol-regression'].flatMap(pack =>
    fs.readdirSync(`testdata/${pack}/problems`, {withFileTypes: true})
      .filter(d => d.isDirectory()).map(d => d.name.replace(/^\d+-/, '')));
  const result = plan(files, {event: kind, knownProblems,
    full: kind === 'workflow_dispatch' || event.pull_request?.labels?.some(l => l.name === 'ci:full')});
  const tests = result.unit || result.integration.length > 0 || result.languages.length > 0;
  const output = {
    tests, lint: result.lint, unit: result.unit,
    integration: result.integration.join(' '), languages: result.languages.join(','),
    selection: JSON.stringify(result.selection), regression: result.languages.length > 0,
  };
  for (const [key, value] of Object.entries(output)) fs.appendFileSync(process.env.GITHUB_OUTPUT, `${key}=${value}\n`);
  fs.writeFileSync('test-plan.json', JSON.stringify(result, null, 2) + '\n');
  fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, `## Test plan\n\n\`\`\`json\n${JSON.stringify(result, null, 2)}\n\`\`\`\n`);
  console.log(result);
}
