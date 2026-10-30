// SPDX-License-Identifier: AGPL-3.0-or-later
// Run only against an owned disposable browser started by external-session.
// Usage: node tools/bench/cdp-error-side-effects.mjs <loopback-cdp-port>
const port = Number(process.argv[2]);
if (!Number.isInteger(port) || port < 1 || port > 65535) {
  console.error('A loopback CDP port is required');
  process.exit(2);
}
const tabs = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
const page = tabs.find(tab => tab.type === 'page');
if (!page?.webSocketDebuggerUrl) throw new Error('No page target');

async function probe(enabled) {
  const ws = new WebSocket(page.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => {
    ws.onopen = resolve;
    ws.onerror = () => reject(new Error('CDP connection failed'));
  });
  let id = 0;
  const pending = new Map();
  let paused;
  ws.onmessage = event => {
    const response = JSON.parse(event.data);
    if (response.method === 'Debugger.paused' && paused) {
      paused(response.params);
      paused = undefined;
    }
    const item = pending.get(response.id);
    if (!item) return;
    pending.delete(response.id);
    clearTimeout(item.timer);
    response.error ? item.reject(new Error(response.error.message)) : item.resolve(response.result);
  };
  function send(method, params = {}) {
    return new Promise((resolve, reject) => {
      const request = ++id;
      const timer = setTimeout(() => {
        pending.delete(request);
        reject(new Error(`CDP timeout: ${method}`));
      }, 10000);
      pending.set(request, {resolve, reject, timer});
      ws.send(JSON.stringify({id: request, method, params}));
    });
  }
  async function evaluate(expression) {
    const result = await send('Runtime.evaluate', {expression, returnByValue: true});
    if (result.exceptionDetails) throw new Error('Page probe threw an exception');
    return result.result.value;
  }
  try {
    await send('Page.navigate', {url: 'about:blank'});
    if (enabled) await send('Runtime.enable');
    await evaluate(`(() => {
      delete Error.prepareStackTrace;
      window.__probeError = new Error('owned-regression');
      window.__probeCalls = 0;
      Error.prepareStackTrace = () => { window.__probeCalls++; return 'custom-stack'; };
      console.log(window.__probeError);
    })()`);
    const beforeRead = await evaluate('window.__probeCalls');
    const stack = await evaluate('window.__probeError.stack');
    const afterRead = await evaluate('window.__probeCalls');
    await evaluate('delete Error.prepareStackTrace');
    const ordinary = await send('Runtime.evaluate', {expression: "new Error('ordinary-regression')"});
    const description = ordinary.result.description || '';
    let debuggerPaused = null;
    let explicitInspectorStack = null;
    if (enabled) {
      await send('Debugger.enable');
      const pause = new Promise((resolve, reject) => {
        const timer = setTimeout(() => reject(new Error('Debugger pause timed out')), 10000);
        paused = params => { clearTimeout(timer); resolve(params); };
      });
      const execution = send('Runtime.evaluate', {expression: 'function ownedBreakpoint() { debugger; return 42; } ownedBreakpoint()'});
      const event = await pause;
      debuggerPaused = event.callFrames.some(frame => frame.functionName === 'ownedBreakpoint');
      await send('Debugger.resume');
      const response = await execution;
      debuggerPaused &&= response.result.value === 42;
      await evaluate(`(() => {
        delete Error.prepareStackTrace;
        window.__inspectorError = new Error('inspector-property');
        window.__inspectorCalls = 0;
        Error.prepareStackTrace = () => { window.__inspectorCalls++; return 'inspector-custom'; };
      })()`);
      const remote = await send('Runtime.evaluate', {expression: 'window.__inspectorError'});
      const beforeProperties = await evaluate('window.__inspectorCalls');
      const properties = await send('Runtime.getProperties', {objectId: remote.result.objectId, ownProperties: true});
      const stackProperty = properties.result.find(property => property.name === 'stack');
      const afterProperties = await evaluate('window.__inspectorCalls');
      explicitInspectorStack = beforeProperties === 0 && afterProperties === 1 &&
        stackProperty?.value?.value === 'inspector-custom';
      await evaluate('delete Error.prepareStackTrace');
      await send('Debugger.disable');
    }
    return {mode: enabled ? 'runtime-enabled' : 'minimal', beforeRead, stack, afterRead,
      ordinaryDescription: description.includes('ordinary-regression'), ordinaryStack: description.includes('\n    at '), debuggerPaused, explicitInspectorStack};
  } finally {
    ws.close();
    for (const item of pending.values()) clearTimeout(item.timer);
  }
}

const results = [];
for (const enabled of [false, true]) results.push(await probe(enabled));
console.log(JSON.stringify(results));
if (results.some(r => r.beforeRead !== 0 || r.stack !== 'custom-stack' || r.afterRead !== 1 ||
    !r.ordinaryDescription || !r.ordinaryStack ||
    (r.mode === 'runtime-enabled' && (!r.debuggerPaused || !r.explicitInspectorStack)))) {
  process.exitCode = 1;
}
