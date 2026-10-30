// SPDX-License-Identifier: AGPL-3.0-or-later
// Owned disposable browser only; checks passive Error serialization in iframe
// and an attached worker, not a detector verdict or native Keychain proof.
// Usage: node tools/bench/cdp-error-contexts.mjs <loopback-cdp-port>
const port = Number(process.argv[2]);
if (!Number.isInteger(port) || port < 1 || port > 65535) {
  console.error('A loopback CDP port is required');
  process.exit(2);
}
const targets = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
const page = targets.find(target => target.type === 'page');
if (!page?.webSocketDebuggerUrl) throw new Error('No page target');
const ws = new WebSocket(page.webSocketDebuggerUrl);
await new Promise((resolve, reject) => {
  ws.onopen = resolve;
  ws.onerror = () => reject(new Error('CDP connection failed'));
});
let nextId = 0;
const pending = new Map();
let workerReadyResolve;
let workerReadyReject;
const workerReady = new Promise((resolve, reject) => {
  workerReadyResolve = resolve;
  workerReadyReject = reject;
});
function send(method, params = {}, sessionId) {
  return new Promise((resolve, reject) => {
    const id = ++nextId;
    const timer = setTimeout(() => { pending.delete(id); reject(new Error(`CDP timeout: ${method}`)); }, 10000);
    pending.set(id, { resolve, reject, timer });
    ws.send(JSON.stringify({ id, method, params, ...(sessionId ? { sessionId } : {}) }));
  });
}
ws.onmessage = event => {
  const message = JSON.parse(event.data);
  if (message.method === 'Target.attachedToTarget' && message.params.targetInfo.type === 'worker') {
    const sessionId = message.params.sessionId;
    (async () => {
      await send('Runtime.enable', {}, sessionId);
      await send('Runtime.runIfWaitingForDebugger', {}, sessionId);
      workerReadyResolve();
    })().catch(workerReadyReject);
  }
  const entry = pending.get(message.id);
  if (!entry) return;
  pending.delete(message.id);
  clearTimeout(entry.timer);
  message.error ? entry.reject(new Error('CDP command failed')) : entry.resolve(message.result);
};
async function evaluate(expression, awaitPromise = false) {
  const response = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise });
  if (response.exceptionDetails) throw new Error('Context probe threw');
  return response.result.value;
}
let readyTimer;
try {
  await send('Page.navigate', { url: 'about:blank' });
  await send('Runtime.enable');
  await send('Target.setAutoAttach', { autoAttach: true, waitForDebuggerOnStart: true, flatten: true });
  const frame = await evaluate(`(() => {
    const f = document.createElement('iframe'); document.body.appendChild(f);
    const w = f.contentWindow; let calls = 0;
    w.Error.prepareStackTrace = () => { calls++; return 'iframe-custom'; };
    const e = new w.Error('iframe-test'); w.console.log(e);
    const before = calls; const stack = e.stack; const after = calls;
    f.remove(); return {before, stack, after};
  })()`);
  await evaluate(`(() => {
    const source = \`onmessage = () => { let calls = 0; Error.prepareStackTrace = () => { calls++; return 'worker-custom'; }; const e = new Error('worker-test'); console.log(e); const before = calls; const stack = e.stack; postMessage({before, stack, after: calls}); };\`;
    const url = URL.createObjectURL(new Blob([source], {type: 'text/javascript'}));
    window.__ownedWorker = new Worker(url); URL.revokeObjectURL(url);
    window.__ownedResult = new Promise((resolve, reject) => {
      window.__ownedWorker.onmessage = event => resolve(event.data);
      window.__ownedWorker.onerror = () => reject(new Error('worker failed'));
    });
  })()`);
  await Promise.race([workerReady, new Promise((_, reject) => {
    readyTimer = setTimeout(() => reject(new Error('Worker attachment timed out')), 10000);
  })]);
  clearTimeout(readyTimer);
  await evaluate('window.__ownedWorker.postMessage("run")');
  const worker = await evaluate('window.__ownedResult', true);
  await evaluate('window.__ownedWorker.terminate()');
  console.log(JSON.stringify({ frame, worker }));
  if (frame?.before !== 0 || frame?.after !== 1 || frame?.stack !== 'iframe-custom' ||
      worker?.before !== 0 || worker?.after !== 1 || worker?.stack !== 'worker-custom') {
    process.exitCode = 1;
  }
} finally {
  clearTimeout(readyTimer);
  ws.close();
  for (const entry of pending.values()) clearTimeout(entry.timer);
}
