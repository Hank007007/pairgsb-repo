// Turns a policy text into a flat token list.
//
// Each token is { value, directiveBreak }.  `directiveBreak` marks the token
// after which a new directive begins.  See docs/policy-grammar.md §2.1.
const WHITESPACE_CODE_POINTS = new Set([
  0x20, // SPACE
  0x09, // CHARACTER TABULATION
  0x0b, // LINE TABULATION
  0x0c, // FORM FEED
  0x0a, // LINE FEED
  0x0d, // CARRIAGE RETURN
  0x3000, // IDEOGRAPHIC SPACE
]);

export function tokenize(text) {
  const input = text == null ? '' : String(text);
  const tokens = [];
  let value = '';
  let hasValue = false;

  const finish_value = (directiveBreak) => {
    if (!hasValue) {
      if (directiveBreak) tokens.push({ value: '', directiveBreak: true });
      return;
    }

    tokens.push({ value, directiveBreak });
    value = '';
    hasValue = false;
  };

  for (const character of input) {
    if (character === ';') {
      finish_value(true);
      continue;
    }

    if (WHITESPACE_CODE_POINTS.has(character.codePointAt(0))) {
      finish_value(false);
      continue;
    }

    value += character;
    hasValue = true;
  }

  finish_value(false);
  return tokens;
}
