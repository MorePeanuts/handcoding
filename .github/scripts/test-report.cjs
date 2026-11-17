const fs = require('node:fs');
const files = process.argv.slice(2);
let report = '## Test timings\n\nPackage elapsed times include race instrumentation. These are wall times, not CPU utilization.\n\n| Layer / package | Seconds | Result |\n|---|---:|---|\n';
const tests = [];
for (const file of files) {
  if (!fs.existsSync(file)) continue;
  for (const line of fs.readFileSync(file, 'utf8').split('\n')) {
    let event;
    try { event = JSON.parse(line); } catch { continue; }
    if (!['pass', 'fail'].includes(event.Action)) continue;
    if (event.Test) {
      if (!event.Test.includes('/')) tests.push(event);
    } else report += `| ${file.replace('test-', '').replace('.jsonl', '')} / ${event.Package} | ${event.Elapsed} | ${event.Action} |\n`;
  }
}
report += '\n### Slowest top-level tests\n\n| Test | Seconds |\n|---|---:|\n';
for (const event of tests.sort((a, b) => b.Elapsed - a.Elapsed).slice(0, 10)) {
  report += `| ${event.Test} | ${event.Elapsed} |\n`;
}
fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, report);
