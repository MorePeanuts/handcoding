function check(needs) {
  if (needs.plan.result !== 'success') throw new Error('Test planning failed');
  for (const name of ['test', 'lint']) {
    const planned = needs.plan.outputs[name === 'test' ? 'tests' : 'lint'];
    if (!['true', 'false'].includes(planned)) throw new Error(`Missing or invalid ${name} plan`);
    const expected = planned === 'true' ? 'success' : 'skipped';
    if (needs[name].result !== expected) throw new Error(`${name}: expected ${expected}, got ${needs[name].result}`);
  }
}
module.exports = check;
if (require.main === module) check(JSON.parse(process.env.CI_NEEDS));
