"use strict";
const fs = require("node:fs"),
  path = require("node:path"),
  vm = require("node:vm");
const { Worker } = require("node:worker_threads");
const { createRequire } = require("node:module");
const { parse, scalar, decode, encode, nodes, check } = require("./codec.cjs");
const types = require("./types.cjs");
class Limit extends Error {}
function run(options) {
  const spec = JSON.parse(fs.readFileSync(options.spec, "utf8"));
  const shared = new SharedArrayBuffer(8 + 8 * 5),
    lock = new Int32Array(shared, 0, 2),
    state = new BigInt64Array(shared, 8);
  function enter() {
    while (Atomics.compareExchange(lock, 0, 0, 1) !== 0)
      Atomics.wait(lock, 0, 1);
  }
  function leave() {
    Atomics.store(lock, 0, 0);
    Atomics.notify(lock, 0);
  }
  const limit = BigInt(process.env.HC_TIME_LIMIT_NS || "0");
  state[3] = limit;
  function timed(fn) {
    enter();
    if (limit > 0n && state[0] >= limit) {
      leave();
      throw new Limit();
    }
    state[1] = process.hrtime.bigint();
    leave();
    let value;
    try {
      value = fn();
      check(
        !(value && typeof value.then === "function"),
        "async/Promise solutions are not supported by this problem protocol",
      );
    } finally {
      enter();
      state[0] += process.hrtime.bigint() - state[1];
      state[1] = 0n;
      leave();
    }
    if (limit > 0n && state[0] > limit) throw new Limit();
    return value;
  }
  const localRequire = createRequire(options.original),
    vendorRequire = createRequire(
      path.join(options.dependencies, "hc.cjs"),
    );
  function requireSolution(name) {
    if (name === "lodash" || name.startsWith("@datastructures-js/"))
      return vendorRequire(name);
    return localRequire(name);
  }
  const globals = {
    console,
    process,
    Buffer,
    URL,
    URLSearchParams,
    TextEncoder,
    TextDecoder,
    setTimeout,
    clearTimeout,
    setInterval,
    clearInterval,
    setImmediate,
    clearImmediate,
    queueMicrotask,
    structuredClone,
    require: requireSolution,
    module: { exports: {} },
    __filename: options.original,
    __dirname: path.dirname(options.original),
    ...types,
  };
  globals.exports = globals.module.exports;
  for (const name of [
    "heap",
    "priority-queue",
    "queue",
    "deque",
    "stack",
    "linked-list",
    "set",
  ])
    Object.assign(globals, vendorRequire("@datastructures-js/" + name));
  globals._ = vendorRequire("lodash");
  if (spec.node) {
    globals.Node = types[spec.node];
    globals._Node = types[spec.node];
  }
  // Keep decoded arrays/nodes in the solution realm (instanceof Array/Object).
  Object.assign(globalThis, globals);
  let source = fs.readFileSync(options.source, "utf8");
  const name = spec.kind === "function" ? spec.function.name : spec.class.name;
  check(/^[A-Za-z_$][\w$]*$/.test(name), "invalid entry name");
  let fallback = "module.exports[" + JSON.stringify(name) + "]";
  if (spec.judge.round_trip) {
    const r = spec.judge.round_trip;
    for (const n of [r.encode, r.decode])
      check(/^[A-Za-z_$][\w$]*$/.test(n), "invalid codec entry");
    fallback =
      "class { " +
      r.encode +
      "(v){return " +
      r.encode +
      "(v);} " +
      r.decode +
      "(v){return " +
      r.decode +
      "(v);} }";
  }
  source +=
    "\n;globalThis.__hcEntry = typeof " +
    name +
    ' !== "undefined" ? ' +
    name +
    " : " +
    fallback +
    ";\n";
  if (spec.judge.round_trip && process.env.HC_JS_DEBUG === "1") {
    const encoder = spec.judge.round_trip.encode;
    source +=
      "\n;globalThis.__hcDebugEntry = typeof " +
      encoder +
      ' === "function" ? ' +
      encoder +
      " : globalThis.__hcEntry.prototype[" +
      JSON.stringify(encoder) +
      "];\n";
  }
  new vm.Script(source, { filename: options.source }).runInThisContext();
  const entry = globalThis.__hcEntry;
  check(typeof entry === "function", "missing solution entry " + name);
  if (process.env.HC_JS_DEBUG === "1") {
    // Break at the real function/constructor on its next invocation, without
    // changing the user's source or requiring an export declaration.
    const inspector = require("node:inspector"),
      session = new inspector.Session();
    session.connect();
    session.post("Debugger.enable");
    const target = spec.judge.round_trip ? globalThis.__hcDebugEntry : entry;
    globalThis.__hcDebugTarget = target;
    session.post(
      "Runtime.evaluate",
      { expression: "globalThis.__hcDebugTarget" },
      (e, r) => {
        if (!e && r.result.objectId)
          session.post("Debugger.setBreakpointOnFunctionCall", {
            objectId: r.result.objectId,
          });
      },
    );
  }
  const cases = parse(fs.readFileSync(process.env.HC_CASES, "utf8"));
  const watcher =
    limit > 0n
      ? new Worker(path.join(__dirname, "watch.cjs"), {
          workerData: { state: shared, out: process.env.HC_OUT },
          execArgv: [],
        })
      : null;
  function callFunction(input) {
    const f = spec.function,
      params = f.params || [],
      all = [...params, ...(f.hidden || [])],
      values = Object.create(null),
      original = Object.create(null),
      byName = Object.fromEntries(all.map((p) => [p.name, p]));
    for (const p of all) {
      if (p.ref || p.copy) continue;
      values[p.name] = decode(p.type, input[p.name]);
      if (p.type === "ListNode" || p.type === "TreeNode")
        original[p.name] = nodes(p.type, values[p.name]);
    }
    for (const p of params) {
      const name = p.name;
      if (p.cycle) {
        const pos = values[p.cycle];
        if (pos >= 0) original[name].at(-1).next = original[name][pos];
      }
      if (p.join) {
        const j = p.join,
          skip = values[j.skip],
          at = values[j.at],
          ns = [
            ...original[name].slice(0, skip),
            ...original[j.list].slice(at),
          ];
        if (skip > 0) ns[skip - 1].next = ns[skip] || null;
        values[name] = ns[0] || null;
        original[name] = ns;
      }
    }
    for (const p of params) {
      if (p.copy) {
        values[p.name] = decode(p.type, encode(p.type, values[p.copy]));
        original[p.name] = nodes(p.type, values[p.name]);
      }
    }
    for (const p of params) {
      if (p.ref) {
        const val = scalar("integer", input[p.name]),
          found = original[p.ref].filter((n) => n.val === val);
        check(found.length === 1, "reference node missing or ambiguous");
        values[p.name] = found[0];
      }
    }
    const before = f.returns_copy
      ? new Set(
          nodes(byName[f.returns_copy].type, values[f.returns_copy], false),
        )
      : null;
    const args = params.map((p) => values[p.name]),
      result = timed(() => entry(...args));
    if (before)
      for (const n of nodes(f.returns, result, false))
        check(!before.has(n), "returned copy shares a node with input");
    if (spec.judge.output) {
      const p = byName[spec.judge.output];
      let out = values[p.name];
      if (spec.judge.output_length) {
        const k = scalar("long", result);
        check(Array.isArray(out), "expected array result");
        check(
          k >= 0 && k <= out.length,
          "returned length outside output array",
        );
        // Only the prefix belongs to the output; its tail may not be encodable.
        out = out.slice(0, k);
      }
      return encode(p.type, out);
    }
    if (f.returns_ref) {
      if (result === null) return null;
      const ns = original[f.returns_ref],
        at = ns.indexOf(result);
      check(at >= 0, "returned node is outside input structure");
      return byName[f.returns_ref].type === "ListNode"
        ? at
        : scalar("integer", result.val);
    }
    return encode(f.returns, result);
  }
  const methods = new Map((spec.class?.methods || []).map((m) => [m.name, m]));
  function args(ps, vs) {
    check(vs.length === (ps || []).length, "wrong argument count");
    return (ps || []).map((p, i) => decode(p.type, vs[i]));
  }
  function callClass(input) {
    const c = spec.class,
      round = spec.judge.round_trip;
    if (round) {
      const enc = methods.get(round.encode),
        dec = methods.get(round.decode),
        value = decode(enc.params[0].type, input[enc.params[0].name]),
        first = timed(() => new entry()),
        text = timed(() => first[enc.name](value));
      scalar("string", text);
      const second =
        round.instances === "same" ? first : timed(() => new entry());
      return encode(
        dec.returns,
        timed(() => second[dec.name](text)),
      );
    }
    const argv = args(c.constructor, input.args[0]),
      obj = timed(() => new entry(...argv)),
      out = [null];
    for (let i = 1; i < input.ops.length; i++) {
      const name = input.ops[i],
        m = methods.get(name);
      check(m && typeof obj[name] === "function", "unknown method " + name);
      const argv = args(m.params, input.args[i]);
      out.push(
        encode(
          m.returns,
          timed(() => obj[name](...argv)),
        ),
      );
    }
    return out;
  }
  for (const c of cases) {
    enter();
    state[2] = BigInt(c.id.text);
    state[4] = state[0];
    leave();
    const result = { id: scalar("long", c.id) };
    try {
      result.output =
        spec.kind === "function" ? callFunction(c.input) : callClass(c.input);
    } catch (e) {
      if (e instanceof Limit) result.time_limit = true;
      else result.error = String(e?.stack || e);
    }
    enter();
    result.elapsed_ns = Number(state[0] - state[4]);
    fs.appendFileSync(process.env.HC_OUT, JSON.stringify(result) + "\n");
    leave();
    if (result.time_limit) break;
  }
  if (watcher) watcher.terminate();
  // A completed batch owns its event loop: solution timers must not keep it alive.
  // Flush pipe-backed console output before terminating user-owned timers.
  process.stdout.write("", () =>
    process.stderr.write("", () => process.exit(0)),
  );
}
module.exports = { run };
