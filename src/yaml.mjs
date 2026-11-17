// Minimal YAML subset reader used for pipeline files.
// Supports block mappings, block sequences, flow sequences/mappings,
// quoted and plain scalars, and literal/folded block scalars.

export function parseYaml(source) {
  const rawLines = String(source).replace(/^﻿/, '').split(/\r?\n/);
  const lines = [];

  for (let index = 0; index < rawLines.length; index += 1) {
    const raw = rawLines[index];
    if (/^\s*$/.test(raw)) continue;
    const indent = raw.match(/^ */)[0].length;
    if (raw[indent] === '\t') {
      throw new Error(`YAML parse error on line ${index + 1}: tabs are not allowed`);
    }
    const text = stripComment(raw.slice(indent)).replace(/\s+$/, '');
    if (text === '') continue;
    lines.push({ indent, text, line: index + 1 });
  }

  if (lines.length === 0) return null;

  let cursor = 0;

  const parseBlock = (indent) => {
    if (cursor >= lines.length) return null;
    if (isSequenceEntry(lines[cursor].text)) return parseSequence(lines[cursor].indent);
    return parseMapping(lines[cursor].indent);
  };

  const parseMapping = (indent) => {
    const result = {};
    while (cursor < lines.length) {
      const entry = lines[cursor];
      if (entry.indent < indent) break;
      if (entry.indent > indent) {
        throw new Error(`YAML parse error on line ${entry.line}: unexpected indentation`);
      }
      if (isSequenceEntry(entry.text)) break;

      const pair = splitKey(entry.text);
      if (!pair) {
        throw new Error(`YAML parse error on line ${entry.line}: expected "key: value"`);
      }
      cursor += 1;

      if (pair.rest === '') {
        if (cursor < lines.length && lines[cursor].indent > indent) {
          result[pair.key] = parseBlock(lines[cursor].indent);
        } else {
          result[pair.key] = null;
        }
      } else if (/^[|>][+-]?$/.test(pair.rest)) {
        result[pair.key] = parseBlockScalar(indent, pair.rest, rawLines, entry.line);
      } else {
        result[pair.key] = parseInline(pair.rest, entry.line);
      }
    }
    return result;
  };

  const parseSequence = (indent) => {
    const result = [];
    while (cursor < lines.length) {
      const entry = lines[cursor];
      if (entry.indent !== indent || !isSequenceEntry(entry.text)) break;

      const afterDash = entry.text.slice(1);
      const spaces = afterDash.match(/^ */)[0].length;
      const rest = afterDash.slice(spaces).replace(/\s+$/, '');
      const contentIndent = indent + 1 + spaces;

      if (rest === '') {
        cursor += 1;
        if (cursor < lines.length && lines[cursor].indent > indent) {
          result.push(parseBlock(lines[cursor].indent));
        } else {
          result.push(null);
        }
        continue;
      }

      const pair = splitKey(rest);
      if (pair) {
        lines[cursor] = { indent: contentIndent, text: rest, line: entry.line };
        result.push(parseMapping(contentIndent));
      } else {
        cursor += 1;
        result.push(parseInline(rest, entry.line));
      }
    }
    return result;
  };

  const parseBlockScalar = (keyIndent, header, raw, keyLine) => {
    const style = header[0];
    const chomp = header.length > 1 ? header[1] : '';
    const collected = [];
    let contentIndent = null;
    let rawIndex = keyLine;
    let lastConsumedLine = keyLine;

    while (rawIndex < raw.length) {
      const rawText = raw[rawIndex];
      if (/^\s*$/.test(rawText)) {
        if (contentIndent !== null) collected.push('');
        rawIndex += 1;
        lastConsumedLine = rawIndex;
        continue;
      }
      const lineIndent = rawText.match(/^ */)[0].length;
      if (lineIndent <= keyIndent) break;
      if (contentIndent === null) contentIndent = lineIndent;
      collected.push(rawText.slice(Math.min(contentIndent, lineIndent)));
      rawIndex += 1;
      lastConsumedLine = rawIndex;
    }

    while (collected.length > 0 && collected[collected.length - 1] === '') {
      collected.pop();
    }
    while (cursor < lines.length && lines[cursor].line <= lastConsumedLine) {
      cursor += 1;
    }

    let value;
    if (style === '|') {
      value = collected.join('\n');
    } else {
      value = collected.reduce((acc, line, position) => {
        if (position === 0) return line;
        if (line === '') return `${acc}\n`;
        if (acc.endsWith('\n')) return acc + line;
        return `${acc} ${line}`;
      }, '');
    }

    if (chomp === '-') return value;
    if (chomp === '+') {
      return collected.length > 0 ? `${value}\n` : '';
    }
    return value === '' ? '' : `${value}\n`;
  };

  const value = parseBlock(lines[0].indent);
  if (cursor !== lines.length) {
    throw new Error(`YAML parse error on line ${lines[cursor].line}: could not parse the remainder`);
  }
  return value;
}

function isSequenceEntry(text) {
  return text === '-' || text.startsWith('- ');
}

function stripComment(text) {
  let quote = null;
  for (let index = 0; index < text.length; index += 1) {
    const char = text[index];
    if (quote) {
      if (char === quote) {
        if (quote === "'" && text[index + 1] === "'") {
          index += 1;
        } else {
          quote = null;
        }
      } else if (quote === '"' && char === '\\') {
        index += 1;
      }
      continue;
    }
    if (char === "'" || char === '"') {
      quote = char;
      continue;
    }
    if (char === '#' && (index === 0 || /\s/.test(text[index - 1]))) {
      return text.slice(0, index);
    }
  }
  return text;
}

function splitKey(text) {
  let quote = null;
  let depth = 0;
  for (let index = 0; index < text.length; index += 1) {
    const char = text[index];
    if (quote) {
      if (char === quote) {
        if (quote === "'" && text[index + 1] === "'") {
          index += 1;
        } else {
          quote = null;
        }
      } else if (quote === '"' && char === '\\') {
        index += 1;
      }
      continue;
    }
    if (char === "'" || char === '"') {
      quote = char;
      continue;
    }
    if (char === '[' || char === '{') {
      depth += 1;
      continue;
    }
    if (char === ']' || char === '}') {
      depth -= 1;
      continue;
    }
    if (char === ':' && depth === 0) {
      const next = text[index + 1];
      if (next === undefined || /\s/.test(next)) {
        return {
          key: parseScalar(text.slice(0, index).trim()),
          rest: text.slice(index + 1).trim(),
        };
      }
    }
  }
  return null;
}

function parseInline(text, line) {
  const trimmed = text.trim();
  if (trimmed.startsWith('[')) {
    if (!trimmed.endsWith(']')) {
      throw new Error(`YAML parse error on line ${line}: unterminated flow sequence`);
    }
    const inner = trimmed.slice(1, -1).trim();
    if (inner === '') return [];
    return splitFlow(inner).map((part) => parseInline(part, line));
  }
  if (trimmed.startsWith('{')) {
    if (!trimmed.endsWith('}')) {
      throw new Error(`YAML parse error on line ${line}: unterminated flow mapping`);
    }
    const inner = trimmed.slice(1, -1).trim();
    const result = {};
    if (inner === '') return result;
    for (const part of splitFlow(inner)) {
      const pair = splitKey(part);
      if (!pair) {
        throw new Error(`YAML parse error on line ${line}: expected "key: value"`);
      }
      result[pair.key] = parseInline(pair.rest, line);
    }
    return result;
  }
  return parseScalar(trimmed);
}

function splitFlow(text) {
  const parts = [];
  let quote = null;
  let depth = 0;
  let start = 0;
  for (let index = 0; index < text.length; index += 1) {
    const char = text[index];
    if (quote) {
      if (char === quote) {
        if (quote === "'" && text[index + 1] === "'") index += 1;
        else quote = null;
      } else if (quote === '"' && char === '\\') {
        index += 1;
      }
      continue;
    }
    if (char === "'" || char === '"') {
      quote = char;
      continue;
    }
    if (char === '[' || char === '{') depth += 1;
    else if (char === ']' || char === '}') depth -= 1;
    else if (char === ',' && depth === 0) {
      parts.push(text.slice(start, index).trim());
      start = index + 1;
    }
  }
  parts.push(text.slice(start).trim());
  return parts.filter((part) => part !== '');
}

function parseScalar(text) {
  const trimmed = text.trim();
  if (trimmed === '') return '';
  if (trimmed.startsWith('"') && trimmed.endsWith('"') && trimmed.length >= 2) {
    try {
      return JSON.parse(trimmed);
    } catch {
      return trimmed.slice(1, -1);
    }
  }
  if (trimmed.startsWith("'") && trimmed.endsWith("'") && trimmed.length >= 2) {
    return trimmed.slice(1, -1).replace(/''/g, "'");
  }
  if (trimmed === 'null' || trimmed === 'Null' || trimmed === 'NULL' || trimmed === '~') {
    return null;
  }
  if (trimmed === 'true' || trimmed === 'True' || trimmed === 'TRUE') return true;
  if (trimmed === 'false' || trimmed === 'False' || trimmed === 'FALSE') return false;
  if (/^-?(?:0|[1-9]\d*)(?:\.\d+)?$/.test(trimmed)) return Number(trimmed);
  return trimmed;
}
