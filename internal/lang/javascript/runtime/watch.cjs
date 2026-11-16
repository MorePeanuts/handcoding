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
  const [spent, start, id, limit, base] = state;
  if (
    start > 0n &&
    limit > 0n &&
    spent + process.hrtime.bigint() - start > limit
  ) {
    fs.appendFileSync(
      workerData.out,
      JSON.stringify({
        id: Number(id),
        elapsed_ns: Number(spent + process.hrtime.bigint() - start - base),
        time_limit: true,
      }) + "\n",
    );
    process.kill(process.pid, "SIGKILL");
  }
  leave();
}, 1);
