import { createServer } from 'node:http';
import { pathToFileURL } from 'node:url';
import { handleInvite } from './invite.js';

const MAX_BODY_BYTES = 1024 * 1024;

/**
 * Read a request body as UTF-8 text.
 *
 * @param {import('node:http').IncomingMessage} req
 * @returns {Promise<string>}
 */
function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let size = 0;
    req.on('data', (chunk) => {
      size += chunk.length;
      if (size > MAX_BODY_BYTES) {
        reject(new Error('request body too large'));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
    req.on('error', reject);
  });
}

/**
 * Send a JSON response.
 *
 * @param {import('node:http').ServerResponse} res
 * @param {number} status
 * @param {unknown} body
 */
function json(res, status, body) {
  const text = JSON.stringify(body);
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': Buffer.byteLength(text),
  });
  res.end(text);
}

/**
 * Create the HTTP server used by the meeting service.
 *
 * @returns {import('node:http').Server}
 */
export function createInviteServer() {
  return createServer(async (req, res) => {
    if (req.method === 'GET' && req.url === '/health') {
      json(res, 200, { status: 'ok' });
      return;
    }
    if (req.method === 'POST' && req.url === '/invite') {
      try {
        const raw = await readBody(req);
        const payload = JSON.parse(raw);
        const result = handleInvite(payload);
        json(res, 200, {
          boundary: result.boundary,
          busyMinutes: result.busyMinutes,
          occurrences: result.occurrences,
          ics: result.ics,
          message: result.envelope,
        });
      } catch (error) {
        json(res, 400, { error: error.message });
      }
      return;
    }
    json(res, 404, { error: 'not found' });
  });
}

/**
 * Start the server on the requested port.
 *
 * @param {number} [port]
 * @returns {import('node:http').Server}
 */
export function startServer(port = 8080) {
  const server = createInviteServer();
  server.listen(port, () => {
    const address = server.address();
    const actual = typeof address === 'object' && address ? address.port : port;
    process.stdout.write(`invitefence listening on http://127.0.0.1:${actual}\r\n`);
  });
  return server;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const port = Number.parseInt(process.env.PORT || '8080', 10);
  startServer(port);
}