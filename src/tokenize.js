// A small, dependency-free tokenizer for the SVG documents the avatar pipeline
// accepts. It produces a flat token stream; the walker in sanitize.js turns it
// back into an allowlisted document.

const NAME_END = new Set([' ', '\t', '\n', '\r', '\f', '/', '>', '=']);

function isSpace(ch) {
  return ch === ' ' || ch === '\t' || ch === '\n' || ch === '\r' || ch === '\f';
}

function skipSpace(input, i) {
  while (i < input.length && isSpace(input[i])) i += 1;
  return i;
}

// Reads a start or end tag beginning at `<`. Returns [token, nextIndex].
function readTag(input, start) {
  let i = start + 1;
  const isEnd = input[i] === '/';
  if (isEnd) i += 1;

  const nameStart = i;
  while (i < input.length && !NAME_END.has(input[i])) i += 1;
  const name = input.slice(nameStart, i);

  const attrs = [];
  let selfClosing = false;
  let ended = false;

  while (i < input.length) {
    i = skipSpace(input, i);
    if (i >= input.length) break;

    const ch = input[i];
    if (ch === '>') {
      i += 1;
      ended = true;
      break;
    }
    if (ch === '/') {
      if (input[i + 1] === '>') {
        selfClosing = true;
        i += 2;
        ended = true;
        break;
      }
      i += 1;
      continue;
    }

    const attrStart = i;
    while (i < input.length && !NAME_END.has(input[i])) i += 1;
    const attrName = input.slice(attrStart, i);
    if (attrName === '') {
      i += 1;
      continue;
    }

    let value = null;
    let j = skipSpace(input, i);
    if (input[j] === '=') {
      j = skipSpace(input, j + 1);
      const quote = input[j];
      if (quote === '"' || quote === "'") {
        const valueStart = j + 1;
        let k = valueStart;
        while (k < input.length && input[k] !== quote) {
          if (input[k] === '>') break;
          k += 1;
        }
        value = input.slice(valueStart, k);
        if (input[k] === '>') {
          i = k + 1;
          ended = true;
          break;
        }
        i = k + 1;
      } else {
        const valueStart = j;
        while (j < input.length && !isSpace(input[j]) && input[j] !== '>') j += 1;
        value = input.slice(valueStart, j);
        i = j;
      }
    }

    attrs.push({ name: attrName, value });
  }

  const token = {
    type: isEnd ? 'endTag' : 'startTag',
    name,
    attrs: isEnd ? [] : attrs,
    selfClosing: isEnd ? false : selfClosing,
    raw: input.slice(start, i),
    complete: ended,
  };
  return [token, i];
}

function readComment(input, start) {
  const close = input.indexOf('-->', start + 4);
  const end = close === -1 ? input.length : close + 3;
  return [{ type: 'comment', raw: input.slice(start, end) }, end];
}

function readDoctype(input, start) {
  // DOCTYPE may carry an internal subset in [...]; find the terminating `>`.
  let i = start + 2;
  let depth = 0;
  let quote = null;
  while (i < input.length) {
    const ch = input[i];
    if (quote) {
      if (ch === quote) quote = null;
    } else if (ch === '"' || ch === "'") {
      quote = ch;
    } else if (ch === '[') {
      depth += 1;
    } else if (ch === ']') {
      depth = Math.max(0, depth - 1);
    } else if (ch === '>' && depth === 0) {
      i += 1;
      break;
    }
    i += 1;
  }
  return [{ type: 'doctype', raw: input.slice(start, i) }, i];
}

function readProcessingInstruction(input, start) {
  const close = input.indexOf('?>', start + 2);
  const end = close === -1 ? input.length : close + 2;
  return [{ type: 'pi', raw: input.slice(start, end) }, end];
}

function readCdata(input, start) {
  const close = input.indexOf(']]>', start + 9);
  const end = close === -1 ? input.length : close + 3;
  return [{ type: 'cdata', raw: input.slice(start, end) }, end];
}

export function tokenize(input) {
  const tokens = [];
  let i = 0;
  let textStart = 0;

  while (i < input.length) {
    if (input[i] !== '<') {
      i += 1;
      continue;
    }

    if (i > textStart) {
      tokens.push({ type: 'text', raw: input.slice(textStart, i) });
    }

    const rest = input.slice(i, i + 9);
    let next;
    if (rest.startsWith('<!--')) {
      const [token, end] = readComment(input, i);
      tokens.push(token);
      next = end;
    } else if (/^<!doctype/i.test(input.slice(i, i + 9))) {
      const [token, end] = readDoctype(input, i);
      tokens.push(token);
      next = end;
    } else if (rest.startsWith('<![CDATA[')) {
      const [token, end] = readCdata(input, i);
      tokens.push(token);
      next = end;
    } else if (rest.startsWith('<?')) {
      const [token, end] = readProcessingInstruction(input, i);
      tokens.push(token);
      next = end;
    } else if (input[i + 1] === '!') {
      const [token, end] = readDoctype(input, i);
      token.type = 'bang';
      tokens.push(token);
      next = end;
    } else {
      const [token, end] = readTag(input, i);
      tokens.push(token);
      next = end;
    }

    i = next;
    textStart = i;
  }

  if (textStart < input.length) {
    tokens.push({ type: 'text', raw: input.slice(textStart) });
  }
  return tokens;
}
