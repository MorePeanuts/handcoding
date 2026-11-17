"use strict";
const { workerData } = require("node:worker_threads");
const fs = require("node:fs");
const lock = new Int32Array(workerData.state, 0, 2),
  state = new BigInt64Array(workerData.state, 8);
function enter() {
  while (Atomics.compareExchange(lock, 0, 0, 1) !== 0) Atomics.wait(lock, 0, 1);
}
function leave() {
  Atomics.store(lock, 0, 0);
  Atomics.notify(lock, 0);
}
setInterval(() => {
  enter();
  const spent = Atomics.load(state, 0),
    start = Atomics.load(state, 1),
    now = process.hrtime.bigint(),
    [, , id, limit, base] = state;
  // A call may have ended while we sampled the clock. Retry on the next
  // tick rather than charge decoding/output time to that finished call.
  const active = start === Atomics.load(state, 1);
  if (
    active &&
    start > 0n &&
    limit > 0n &&
    spent + now - start > limit
  ) {
    fs.appendFileSync(
      workerData.out,
      JSON.stringify({
        id: Number(id),
        elapsed_ns: Number(spent + now - start - base),
        time_limit: true,
      }) + "\n",
    );
    process.kill(process.pid, "SIGKILL");
  }
  leave();
}, 1);
