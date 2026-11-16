"use strict";
// Small terminal frontend to Node's inspector. Source maps are resolved here
// because Node's built-in `inspect` CLI displays emitted JS for TypeScript.
const fs = require("node:fs"),
  path = require("node:path"),
  { fileURLToPath } = require("node:url");
const { spawn } = require("node:child_process"),
  readline = require("node:readline");
const child = spawn(
  process.argv[2],
  [
    "--enable-source-maps",
    "--stack-size=8192",
    "--inspect-brk=127.0.0.1:0",
    process.argv[3],
  ],
  { env: process.env, stdio: ["ignore", "pipe", "pipe"] },
);
let socket,
  seq = 0,
  frames = [],
  initial = true,
  closed = false,
  stderr = "";
const pending = new Map(),
  scripts = new Map();
const input = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  prompt: "debug> ",
});
function finish() {
  if (closed) return;
  closed = true;
  child.kill();
  socket?.close();
  input.close();
}
input.on("close", finish);
process.on("SIGINT", finish);
process.on("exit", () => child.kill());
child.stdout.pipe(process.stdout);
child.on("error", (e) => {
  console.error(e.message);
  finish();
  process.exitCode = 1;
});
child.on("exit", (code) => {
  finish();
  process.exitCode = code || 0;
});
function request(method, params = {}) {
  return new Promise((resolve, reject) => {
    const id = ++seq;
    pending.set(id, { resolve, reject });
    socket.send(JSON.stringify({ id, method, params }));
  });
}
function filename(url) {
  try {
    return url.startsWith("file:") ? fileURLToPath(url) : url;
  } catch {
    return url;
  }
}
function sourceMap(script) {
  if (script.map !== undefined) return script.map;
  script.map = null;
  const file = filename(script.url);
  if (!script.sourceMapURL || !file) return null;
  try {
    const mapFile = path.resolve(path.dirname(file), script.sourceMapURL),
      data = JSON.parse(fs.readFileSync(mapFile, "utf8"));
    const entries = [];
    let src = 0,
      line = 0,
      col = 0,
      name = 0;
    const alphabet =
      "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    data.mappings.split(";").forEach((row, generatedLine) => {
      let generatedColumn = 0;
      for (const segment of row.split(",")) {
        if (!segment) continue;
        const values = [];
        let v = 0,
          shift = 0;
        for (const ch of segment) {
          const n = alphabet.indexOf(ch);
          v |= (n & 31) << shift;
          if (n & 32) {
            shift += 5;
            continue;
          }
          values.push(v & 1 ? -(v >> 1) : v >> 1);
          v = 0;
          shift = 0;
        }
        generatedColumn += values[0];
        if (values.length < 4) continue;
        src += values[1];
        line += values[2];
        col += values[3];
        if (values.length === 5) name += values[4];
        entries.push({
          generatedLine,
          generatedColumn,
          file: path.resolve(
            path.dirname(mapFile),
            data.sourceRoot || "",
            data.sources[src],
          ),
          line,
          column: col,
        });
      }
    });
    script.map = entries;
  } catch {
    /* Ordinary JS and files without a readable map still debug normally. */
  }
  return script.map;
}
function location(loc) {
  const script = scripts.get(loc.scriptId);
  if (!script)
    return {
      file: "<unknown>",
      line: loc.lineNumber,
      column: loc.columnNumber || 0,
    };
  const candidates = sourceMap(script)?.filter(
    (x) =>
      x.generatedLine === loc.lineNumber &&
      x.generatedColumn <= (loc.columnNumber || 0),
  );
  return (
    candidates?.at(-1) || {
      file: filename(script.url),
      line: loc.lineNumber,
      column: loc.columnNumber || 0,
    }
  );
}
function listing(where, radius = 2) {
  try {
    const lines = fs.readFileSync(where.file, "utf8").split("\n");
    for (
      let i = Math.max(0, where.line - radius);
      i < Math.min(lines.length, where.line + radius + 1);
      i++
    )
      console.log(`${i === where.line ? ">" : " "} ${i + 1} ${lines[i]}`);
  } catch {}
}
function display(params) {
  frames = params.callFrames;
  const at = location(frames[0].location);
  console.log(
    `\n${initial ? "Break on start" : "break"} in ${at.file}:${at.line + 1}`,
  );
  initial = false;
  listing(at);
  input.prompt();
}
function connect(url) {
  socket = new WebSocket(url);
  socket.addEventListener("message", (event) => {
    const msg = JSON.parse(event.data);
    if (msg.id) {
      const p = pending.get(msg.id);
      if (p) {
        pending.delete(msg.id);
        if (msg.error) p.reject(new Error(msg.error.message));
        else p.resolve(msg.result);
      }
      return;
    }
    if (msg.method === "Debugger.scriptParsed")
      scripts.set(msg.params.scriptId, msg.params);
    if (msg.method === "Debugger.paused") display(msg.params);
  });
  socket.addEventListener("open", async () => {
    try {
      await request("Runtime.enable");
      await request("Debugger.enable");
      await request("Runtime.runIfWaitingForDebugger");
    } catch (e) {
      console.error(e.message);
      finish();
    }
  });
  socket.addEventListener("error", () => {
    console.error("Inspector connection failed");
    finish();
  });
}
child.stderr.on("data", (chunk) => {
  stderr += chunk.toString();
  let newline;
  while ((newline = stderr.indexOf("\n")) >= 0) {
    const line = stderr.slice(0, newline);
    stderr = stderr.slice(newline + 1);
    if (line.startsWith("Debugger listening on ws://")) {
      if (!socket) connect(line.slice("Debugger listening on ".length).trim());
    } else if (line === "Waiting for the debugger to disconnect...") {
      socket?.close();
    } else if (
      line !== "Debugger attached." &&
      !line.startsWith("For help, see: https://nodejs.org/")
    ) {
      process.stderr.write(line + "\n");
    }
  }
});
child.stderr.on("end", () => {
  if (stderr) process.stderr.write(stderr);
});

async function breakpoint(command) {
  const match = command.match(
    /^(?:sb|break)\s*(?:\(\s*)?(?:(?:"([^"]+)"|'([^']+)')\s*,\s*)?(\d+)\s*\)?$/,
  );
  if (!match) throw new Error('Use sb(line) or sb("file", line)');
  const file = match[1] || match[2] || location(frames[0].location).file,
    line = Number(match[3]) - 1;
  for (const [scriptId, script] of scripts) {
    const mapped = sourceMap(script)?.find(
      (x) =>
        (x.file === file || x.file.endsWith("/" + file)) && x.line === line,
    );
    if (mapped) {
      await request("Debugger.setBreakpoint", {
        location: {
          scriptId,
          lineNumber: mapped.generatedLine,
          columnNumber: mapped.generatedColumn,
        },
      });
      console.log(`Breakpoint set at ${mapped.file}:${line + 1}`);
      return;
    }
    if (
      filename(script.url) === file ||
      filename(script.url).endsWith("/" + file)
    ) {
      await request("Debugger.setBreakpoint", {
        location: { scriptId, lineNumber: line },
      });
      console.log(`Breakpoint set at ${file}:${line + 1}`);
      return;
    }
  }
  throw new Error("Source not loaded; continue to the solution entry first");
}
input.on("line", async (line) => {
  const command = line.trim();
  try {
    if ([".exit", "quit", "q", "exit"].includes(command)) {
      finish();
      return;
    }
    const actions = {
      c: "resume",
      cont: "resume",
      continue: "resume",
      n: "stepOver",
      next: "stepOver",
      s: "stepInto",
      step: "stepInto",
      o: "stepOut",
      out: "stepOut",
    };
    if (actions[command]) {
      await request("Debugger." + actions[command]);
      return;
    }
    if (command === "help" || command === "?") {
      console.log(
        'cont/c · next/n · step/s · out/o · list · exec <expression> · backtrace · sb(line) · sb("file", line) · quit',
      );
    } else if (command === "list" || command === "l")
      listing(location(frames[0].location), 5);
    else if (command === "backtrace" || command === "bt")
      frames.forEach((f, i) => {
        const at = location(f.location);
        console.log(
          `${i}: ${f.functionName || "<anonymous>"} ${at.file}:${at.line + 1}`,
        );
      });
    else if (command.startsWith("sb") || command.startsWith("break"))
      await breakpoint(command);
    else if (command.startsWith("exec ")) {
      checkPaused();
      const r = await request("Debugger.evaluateOnCallFrame", {
        callFrameId: frames[0].callFrameId,
        expression: command.slice(5),
        generatePreview: true,
      });
      console.log(
        "> " +
          (r.exceptionDetails?.text ||
            r.result.description ||
            JSON.stringify(r.result.value)),
      );
    } else if (command) console.log("Unknown command; type help.");
  } catch (e) {
    console.error(e.message);
  }
  input.prompt();
});
function checkPaused() {
  if (!frames.length) throw new Error("Debugger is not paused");
}
