import http from 'node:http';

/**
 * Start an HTTP server on an ephemeral port.
 *
 * @param {(req: http.IncomingMessage, res: http.ServerResponse, body: Buffer) => void} handler
 * @returns {Promise<{url: string, port: number, close: () => Promise<void>}>}
 */
export async function startServer(handler) {
  const server = http.createServer((req, res) => {
    const chunks = [];
    req.on('data', (chunk) => chunks.push(chunk));
    req.on('end', () => handler(req, res, Buffer.concat(chunks)));
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address();
  return {
    url: `http://127.0.0.1:${port}`,
    port,
    close: () => new Promise((resolve) => server.close(resolve)),
  };
}

/**
 * Perform an HTTP request and collect the whole response.
 *
 * @returns {Promise<{status: number, headers: object, body: string, json: any}>}
 */
export function request(url, options = {}) {
  const target = new URL(url);
  return new Promise((resolve, reject) => {
    const req = http.request(
      {
        hostname: target.hostname,
        port: target.port,
        path: `${target.pathname}${target.search}`,
        method: options.method || 'GET',
        headers: options.headers || {},
      },
      (res) => {
        const chunks = [];
        res.on('data', (chunk) => chunks.push(chunk));
        res.on('end', () => {
          const body = Buffer.concat(chunks).toString('utf8');
          let json = null;
          try {
            json = JSON.parse(body);
          } catch {
            json = null;
          }
          resolve({ status: res.statusCode, headers: res.headers, body, json });
        });
      },
    );
    req.on('error', reject);
    if (options.body !== undefined) req.write(options.body);
    req.end();
  });
}

/** Collect the headers an upstream received, so tests can inspect them. */
export function recordingHandler() {
  const seen = [];
  const handler = (req, res, body) => {
    seen.push({ method: req.method, url: req.url, headers: req.headers, body: body.toString('utf8') });
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ ok: true, path: req.url }));
  };
  handler.seen = seen;
  return handler;
}
