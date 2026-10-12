// SPDX-License-Identifier: AGPL-3.0-or-later
// Local HTTP proxy on 127.0.0.1 that forwards to an authenticated upstream HTTP proxy.
// The browser sees an unauthenticated proxy; the credential never reaches its command line.
// Usage: UPSTREAM=http://user:pass@host:port node proxy-relay.mjs <listen-port>
import net from 'node:net';

const up = new URL(process.env.UPSTREAM);
const auth = 'Basic ' + Buffer.from(`${decodeURIComponent(up.username)}:${decodeURIComponent(up.password)}`).toString('base64');
const listenPort = Number(process.argv[2]);

net.createServer((client) => {
  client.once('data', (head) => {
    const text = head.toString('latin1');
    const headerEnd = text.indexOf('\r\n\r\n');
    if (headerEnd < 0) return client.destroy();
    // Inject Proxy-Authorization after the request line, for CONNECT and plain HTTP alike.
    const firstLineEnd = text.indexOf('\r\n');
    const rewritten = Buffer.concat([
      Buffer.from(text.slice(0, firstLineEnd + 2) + `Proxy-Authorization: ${auth}\r\n`, 'latin1'),
      head.subarray(Buffer.byteLength(text.slice(0, firstLineEnd + 2), 'latin1')),
    ]);
    const upstream = net.connect(Number(up.port), up.hostname, () => {
      upstream.write(rewritten);
      client.pipe(upstream).pipe(client);
    });
    upstream.on('error', () => client.destroy());
    client.on('error', () => upstream.destroy());
  });
}).listen(listenPort, '127.0.0.1', () => console.log(`relay 127.0.0.1:${listenPort} -> ${up.hostname}:${up.port}`));
