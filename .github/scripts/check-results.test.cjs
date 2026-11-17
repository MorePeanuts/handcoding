const {test} = require('node:test');
const assert = require('node:assert/strict');
const check = require('./check-results.cjs');
test('only planned skips pass the required check', () => {
  const needs = (enabled, result) => ({plan: {result: 'success', outputs: {tests: enabled, lint: enabled}}, test: {result}, lint: {result}});
  assert.doesNotThrow(() => check(needs('true', 'success')));
  assert.doesNotThrow(() => check(needs('false', 'skipped')));
  for (const result of ['skipped', 'cancelled', 'failure']) assert.throws(() => check(needs('true', result)));
  assert.throws(() => check(needs(undefined, 'skipped')));
  const failed = needs('false', 'skipped'); failed.plan.result = 'failure';
  assert.throws(() => check(failed));
});
