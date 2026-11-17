import { protocolError } from './errors.js';

const ATTRIBUTE_TYPE = /^[A-Za-z][A-Za-z0-9-]*(;[A-Za-z0-9-]+)*$/;
const HEX = /^[0-9a-fA-F]{2}$/;

function isHexPair(text, index) {
  return index + 2 <= text.length && HEX.test(text.slice(index, index + 2));
}

class FilterReader {
  constructor(text) {
    this.text = text;
    this.index = 0;
  }

  get done() {
    return this.index >= this.text.length;
  }

  peek() {
    return this.text[this.index];
  }

  next() {
    const ch = this.text[this.index];
    this.index += 1;
    return ch;
  }

  skipSpaces() {
    while (!this.done && this.text[this.index] === ' ') this.index += 1;
  }

  expect(ch) {
    if (this.peek() !== ch) {
      throw protocolError(`expected "${ch}" at position ${this.index}`, { position: this.index });
    }
    this.index += 1;
  }

  parseAll() {
    this.skipSpaces();
    const filters = [];
    while (!this.done) {
      filters.push(this.parseFilter());
      this.skipSpaces();
    }
    return filters;
  }

  parseFilter() {
    this.expect('(');
    this.skipSpaces();
    const ch = this.peek();
    if (ch === '&' || ch === '|') {
      this.index += 1;
      const children = [];
      this.skipSpaces();
      while (this.peek() === '(') {
        children.push(this.parseFilter());
        this.skipSpaces();
      }
      this.expect(')');
      return { type: ch === '&' ? 'and' : 'or', children };
    }
    if (ch === '!') {
      this.index += 1;
      this.skipSpaces();
      const child = this.parseFilter();
      this.skipSpaces();
      this.expect(')');
      return { type: 'not', child };
    }
    return this.parseItem();
  }

  parseItem() {
    const start = this.index;
    while (!this.done && this.peek() !== '=' && this.peek() !== '(' && this.peek() !== ')') {
      this.index += 1;
    }
    if (this.peek() !== '=') {
      throw protocolError(`filter item without an operator near position ${start}`, { position: start });
    }
    const marker = this.index - 1;
    const previous = marker >= start ? this.text[marker] : '';
    const operator = previous === '~' || previous === '>' || previous === '<' ? `${previous}=` : '=';
    const attributeEnd = operator === '=' ? this.index : marker;
    const attribute = this.text.slice(start, attributeEnd);
    if (!ATTRIBUTE_TYPE.test(attribute)) {
      throw protocolError(`invalid attribute type "${attribute}"`, { attribute });
    }
    this.index += 1;
    const value = this.readValue();
    this.expect(')');
    return this.buildItem(attribute, operator, value);
  }

  readValue() {
    const segments = [];
    let buffer = '';
    let wildcardCount = 0;
    while (!this.done && this.peek() !== ')') {
      const ch = this.next();
      if (ch === '\\') {
        if (!isHexPair(this.text, this.index)) {
          throw protocolError('invalid escape sequence in filter value', { position: this.index });
        }
        const code = Number.parseInt(this.text.slice(this.index, this.index + 2), 16);
        this.index += 2;
        buffer += String.fromCharCode(code);
        continue;
      }
      if (ch === '*') {
        wildcardCount += 1;
        segments.push({ literal: buffer, wildcardAfter: true });
        buffer = '';
        continue;
      }
      if (ch.charCodeAt(0) < 0x20) {
        throw protocolError('unescaped control character in filter value', {
          position: this.index - 1,
        });
      }
      buffer += ch;
    }
    if (this.done) {
      throw protocolError('unterminated filter value');
    }
    segments.push({ literal: buffer, wildcardAfter: false });
    return { segments, wildcardCount };
  }

  buildItem(attribute, operator, value) {
    const literals = value.segments.map((segment) => segment.literal);
    if (operator !== '=') {
      if (value.wildcardCount > 0) {
        throw protocolError('wildcards are not allowed with this operator', { attribute, operator });
      }
      return { type: 'compare', attribute, operator, value: literals[0] };
    }
    if (value.wildcardCount === 0) {
      return { type: 'equal', attribute, value: literals[0] };
    }
    if (literals.length === 2 && literals[0] === '' && literals[1] === '') {
      return { type: 'present', attribute };
    }
    return {
      type: 'substring',
      attribute,
      initial: literals[0],
      final: literals[literals.length - 1],
      any: literals.slice(1, -1),
    };
  }
}

export function parseFilter(input) {
  const text = String(input ?? '');
  if (text.trim() === '') {
    throw protocolError('empty filter');
  }
  const filters = new FilterReader(text).parseAll();
  if (filters.length === 0) {
    throw protocolError('empty filter');
  }
  if (filters.length === 1) return filters[0];
  return { type: 'and', children: filters };
}
