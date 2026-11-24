import { tokenize } from './tokenize.mjs';
import { HASH_ALGORITHMS, POLICY_KEYWORD_INNER_SET } from './syntax.mjs';
import { PolicyError } from './error.mjs';

const ALLOWED_SCHEMES = new Set(['https']);
const HOSTNAME_LABEL = /^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$/;
const DIRECTIVE_NAME = /^[A-Za-z][A-Za-z0-9-]*$/;
const TEXT_WHITESPACE = /[ \t\v\f\n\r\u3000]+/;

// Parses a tenant policy into { directiveName: [source, ...] }.
//
// `input` is either the text form or the JSON form (an object whose values are
// strings or arrays of strings).
export function parse_policy(input) {
  if (typeof input === 'string') return parse_text(input);
  if (is_plain_object(input)) return parse_document(input);
  throw new PolicyError('policy must be a policy string or a policy object', {
    received: typeof input,
  });
}

function parse_text(text) {
  const occurrences = [];
  let name = null;
  let sources = [];

  const finish_directive = () => {
    if (name === null) return;
    occurrences.push({ name, sources });
    name = null;
    sources = [];
  };

  for (const token of tokenize(text)) {
    if (token.value === '') {
      if (token.directiveBreak) finish_directive();
      continue;
    }

    if (name === null) {
      name = token.value;
      sources = [];
    } else {
      sources.push(token.value);
    }

    if (token.directiveBreak) {
      finish_directive();
    }
  }

  finish_directive();
  return build_policy(occurrences);
}

function parse_document(document) {
  const occurrences = [];
  for (const [name, value] of Object.entries(document)) {
    occurrences.push({ name, sources: document_sources(name, value) });
  }
  return build_policy(occurrences);
}

function document_sources(name, value) {
  if (typeof value === 'string') {
    return value.split(TEXT_WHITESPACE).filter((part) => part !== '');
  }
  if (Array.isArray(value)) {
    return value.map((entry) => {
      if (typeof entry !== 'string') {
        throw new PolicyError(`directive "${name}" has a non-string source`, {
          directive: name,
          value: entry,
        });
      }
      return entry;
    });
  }
  throw new PolicyError(
    `directive "${name}" must be a string or an array of strings`,
    { directive: name, value },
  );
}

function build_policy(occurrences) {
  const directives = {};

  for (const occurrence of occurrences) {
    const rawName = occurrence.name;
    if (typeof rawName !== 'string' || !DIRECTIVE_NAME.test(rawName)) {
      throw new PolicyError(`invalid directive name ${JSON.stringify(rawName)}`, {
        directive: rawName,
      });
    }

    const name = rawName.toLowerCase();
    if (!Object.prototype.hasOwnProperty.call(directives, name)) {
      directives[name] = [];
    }

    for (const rawSource of occurrence.sources) {
      const source = normalize_source(rawSource);
      validate_source_expression(name, source);
      add_unique(directives[name], source);
    }

    validate_none_only(name, directives[name]);
  }

  return directives;
}

export function normalize_source(source) {
  if (
    typeof source === 'string' &&
    source.length >= 2 &&
    source.startsWith("'") &&
    source.endsWith("'")
  ) {
    const inner = source.slice(1, -1);
    if (POLICY_KEYWORD_INNER_SET.has(inner.toLowerCase())) {
      return `'${inner.toLowerCase()}'`;
    }
  }
  return source;
}

function add_unique(sources, source) {
  if (!sources.includes(source)) sources.push(source);
}

function validate_none_only(name, sources) {
  if (sources.includes("'none'") && sources.length > 1) {
    throw new PolicyError(
      `directive "${name}" cannot combine 'none' with another source`,
      { directive: name, source: "'none'", sources: sources.slice() },
    );
  }
}

function is_plain_object(value) {
  return (
    typeof value === 'object' &&
    value !== null &&
    !Array.isArray(value) &&
    (Object.getPrototypeOf(value) === Object.prototype ||
      Object.getPrototypeOf(value) === null)
  );
}

// Checks a single source expression in the context of its directive.
//
// Returns true when the expression is valid.  Throws PolicyError naming the
// directive and the expression otherwise.  See docs/policy-grammar.md §3-§4.
export function validate_source_expression(directive, source) {
  const label = JSON.stringify(directive);
  const fail = (reason) => {
    throw new PolicyError(
      `invalid source expression ${JSON.stringify(source)} in directive ${label}: ${reason}`,
      { directive, source, reason },
    );
  };

  if (typeof source !== 'string' || source === '') {
    fail('source expression is empty');
  }
  if (/[\u0000-\u001f\u007f]/.test(source)) {
    fail('source expression contains a control character');
  }
  if (/[;,]/.test(source)) {
    fail('source expression contains a directive separator');
  }

  if (source.startsWith("'")) {
    if (source.length < 2 || !source.endsWith("'")) {
      fail('quoted source expression is not terminated');
    }
    const inner = source.slice(1, -1);
    if (POLICY_KEYWORD_INNER_SET.has(inner.toLowerCase())) return true;

    const nonce = inner.match(/^nonce-(.+)$/);
    if (nonce) {
      if (!is_base64_value(nonce[1])) fail('nonce is not a valid base64 value');
      return true;
    }

    const hash = inner.match(/^(sha256|sha384|sha512)-(.+)$/);
    if (hash) {
      if (!HASH_ALGORITHMS.includes(hash[1])) fail('unknown hash algorithm');
      if (!is_base64_value(hash[2])) fail('hash is not a valid base64 value');
      return true;
    }

    fail('unknown quoted source expression');
  }

  const scheme_only = source.match(/^([A-Za-z][A-Za-z0-9+.-]*):$/);
  if (scheme_only) {
    const scheme = scheme_only[1].toLowerCase();
    if (!ALLOWED_SCHEMES.has(scheme)) fail(`scheme "${scheme}" is not allowed`);
    return true;
  }

  let rest = source;
  const scheme_prefix = rest.match(/^([A-Za-z][A-Za-z0-9+.-]*):\/\//);
  if (scheme_prefix) {
    const scheme = scheme_prefix[1].toLowerCase();
    if (!ALLOWED_SCHEMES.has(scheme)) fail(`scheme "${scheme}" is not allowed`);
    rest = rest.slice(scheme_prefix[0].length);
  }

  if (rest.startsWith('//')) {
    fail('scheme-relative source expression is not allowed');
  }

  const slash = rest.indexOf('/');
  const authority = slash === -1 ? rest : rest.slice(0, slash);
  const path = slash === -1 ? '' : rest.slice(slash);

  if (authority === '') fail('source expression has no host');
  if (authority.includes('@')) fail('source expression has user information');

  const colon = authority.indexOf(':');
  let host = authority;
  if (colon !== -1) {
    host = authority.slice(0, colon);
    const port = authority.slice(colon + 1);
    if (port.includes(':')) fail('source expression has more than one port');
    if (!/^(?:[0-9]+|\*)$/.test(port)) fail('port must be digits or "*"');
  }

  if (host === '*') {
    fail('wildcard host must use the leading "*." label form');
  }

  if (host.startsWith('*.')) {
    const hostname = host.slice(2);
    if (hostname === '') fail('wildcard host has no hostname');
    if (hostname.includes('*')) fail('wildcard is only allowed as a leading label');
    if (!is_hostname(hostname)) fail('wildcard host is not a valid hostname');
  } else {
    if (host.includes('*')) fail('wildcard is only allowed as a leading label');
    if (!is_hostname(host)) fail('host is not a valid hostname');
  }

  if (path !== '' && !/^\/[\x21-\x7e]*$/.test(path)) {
    fail('path contains an invalid character');
  }

  return true;
}

function is_hostname(value) {
  if (value === '' || value.length > 253) return false;
  return value.split('.').every((label) => HOSTNAME_LABEL.test(label));
}

function is_base64_value(value) {
  if (!/^[A-Za-z0-9+/_-]+={0,2}$/.test(value)) return false;
  const padding = value.endsWith('==') ? 2 : value.endsWith('=') ? 1 : 0;
  const data = padding === 0 ? value : value.slice(0, -padding);
  if (padding > 0) {
    if (value.length % 4 !== 0) return false;
    if (data.length % 4 !== 4 - padding) return false;
  } else if (data.length % 4 === 1) {
    return false;
  }
  return true;
}
