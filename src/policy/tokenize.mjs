// Turns a policy text into a flat token list.
//
// Each token is { value, directiveBreak }.  `directiveBreak` marks the token
// after which a new directive begins.  See docs/policy-grammar.md §2.1.
export function tokenize(text) {
  const input = text == null ? '' : String(text);
  const tokens = [];
  for (const piece of input.split(' ')) {
    if (piece === '') continue;
    const directiveBreak = piece.endsWith(';');
    const value = directiveBreak ? piece.slice(0, -1) : piece;
    tokens.push({ value, directiveBreak });
  }
  return tokens;
}
