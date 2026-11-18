import http from 'node:http';
import { createDatabase } from './db.mjs';
import { handleQuery } from './service.mjs';

const MAX_BODY_BYTES = 1024 * 1024;

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

function sendJson(res, status, body) {
  const payload = JSON.stringify(body);
  res.writeHead(status, {
    'content-type': 'application/json; charset=utf-8',
    'content-length': Buffer.byteLength(payload),
  });
  res.end(payload);
}

export function createQueryServer({ db } = {}) {
  const database = db ?? createDatabase();
  return http.createServer(async (req, res) => {
    try {
      if (req.method === 'GET' && req.url === '/health') {
        sendJson(res, 200, { ok: true });
        return;
      }
      if (req.method === 'POST' && req.url === '/query') {
        const raw = await readBody(req);
        let body;
        try {
          body = JSON.parse(raw.length === 0 ? '{}' : raw);
        } catch {
          sendJson(res, 400, {
            error: { code: 'invalid_query', message: 'request body is not valid JSON' },
          });
          return;
        }
        const { status, body: response } = handleQuery(database, body);
        sendJson(res, status, response);
        return;
      }
      sendJson(res, 404, {
        error: { code: 'not_found', message: 'no route for this request' },
      });
    } catch {
      sendJson(res, 500, {
        error: { code: 'internal_error', message: 'request failed' },
      });
    }
  });
}
