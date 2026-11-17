const RESERVED = new Set([',', '+', '"', '\\', '<', '>', ';', '=']);

function escapeByte(code) {
  return `\\${code.toString(16).padStart(2, '0')}`;
}

function escapeLoneSurrogate(code) {
  return (
    escapeByte(0xe0 | (code >> 12)) +
    escapeByte(0x80 | ((code >> 6) & 0x3f)) +
    escapeByte(0x80 | (code & 0x3f))
  );
}

export function escapeDN(value) {
  if (value === null || value === undefined) return '';
  const text = String(value);
  const lastIndex = text.length - 1;
  let result = '';
  for (let index = 0; index < text.length; index += 1) {
    const code = text.charCodeAt(index);
    const character = text[index];
    if (code >= 0xd800 && code <= 0xdbff) {
      const next = text.charCodeAt(index + 1);
      if (next >= 0xdc00 && next <= 0xdfff) {
        result += character + text[index + 1];
        index += 1;
      } else {
        result += escapeLoneSurrogate(code);
      }
    } else if (code >= 0xdc00 && code <= 0xdfff) {
      result += escapeLoneSurrogate(code);
    } else if (RESERVED.has(character)) {
      result += `\\${character}`;
    } else if (character === ' ' && (index === 0 || index === lastIndex)) {
      result += '\\ ';
    } else if (character === '#' && index === 0) {
      result += '\\#';
    } else if (code <= 0x1f || code === 0x7f) {
      result += escapeByte(code);
    } else {
      result += character;
    }
  }
  return result;
}

export function buildUserDN(username, peopleSuffix) {
  return `uid=${escapeDN(username)},${peopleSuffix}`;
}

export function buildServiceDN(accountName, serviceSuffix) {
  return `cn=${escapeDN(accountName)},${serviceSuffix}`;
}
