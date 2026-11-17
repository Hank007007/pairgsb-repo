"""Pre-storage HTML cleanup for ticket comments.

``sanitize`` walks an HTML fragment once and writes a normalised copy of it.
Element names are matched against the tag policy, attribute values are
filtered by the attribute and URL policies, and inline styles are reduced to
the declarations the style policy permits.
"""

import re
from html import escape, unescape

from .attrs import allowed_attrs
from .model import ALLOWED_TAGS, DROP_CONTENT_TAGS, VOID_TAGS

_TAG_NAME = re.compile(r"[A-Za-z][A-Za-z0-9_.:-]*")
_ATTR = re.compile(
    r"""\s*([^\s"'<>/=]+)"""
    r"""(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>`]+)))?"""
)


def sanitize(html):
    """Return *html* reduced to the tagwall policy."""
    if not html:
        return ""
    return _Scanner(html).run()


class _Scanner:
    def __init__(self, source):
        self.source = source
        self.length = len(source)
        self.pos = 0
        self.out = []
        # Open elements, innermost last.  Each entry is ``(name, emitted)``
        # where ``emitted`` says whether a start tag was written for it.
        self.stack = []

    # -- driver ---------------------------------------------------------

    def run(self):
        while self.pos < self.length:
            lt = self.source.find("<", self.pos)
            if lt < 0:
                self._text(self.source[self.pos:])
                self.pos = self.length
                break
            if lt > self.pos:
                self._text(self.source[self.pos:lt])
                self.pos = lt
            if self.source.startswith("<!--", lt):
                self._comment(lt)
            elif self.source.startswith("<![CDATA[", lt):
                self._cdata(lt)
            elif self.source.startswith("<!", lt) or self.source.startswith("<?", lt):
                self._declaration(lt)
            elif self.source.startswith("</", lt):
                self._end_tag(lt)
            else:
                self._start_tag(lt)
        self._close_to(0)
        return "".join(self.out)

    # -- nodes ----------------------------------------------------------

    def _text(self, text):
        self.out.append(escape(unescape(text), quote=False))

    def _comment(self, start):
        # ``<!-->`` and ``<!--->`` are empty comments that end at ``>``.
        # Anything after them is scanned normally rather than copied through.
        if self.source.startswith("<!-->", start):
            self.pos = start + 5
            return
        if self.source.startswith("<!--->", start):
            self.pos = start + 6
            return
        end = self.source.find("-->", start + 4)
        if end >= 0:
            self.pos = end + 3
            return
        # No ``-->`` terminator: the comment runs to the end of the fragment.
        self.pos = self.length

    def _cdata(self, start):
        end = self.source.find("]]>", start + 9)
        self.pos = self.length if end < 0 else end + 3

    def _declaration(self, start):
        if self.source.startswith("<?", start):
            end = self.source.find("?>", start + 2)
            self.pos = self.length if end < 0 else end + 2
            return
        quote = None
        index = start + 2
        while index < self.length:
            char = self.source[index]
            if quote is not None:
                if char == quote:
                    quote = None
            elif char in ("'", '"'):
                quote = char
            elif char == ">":
                self.pos = index + 1
                return
            index += 1
        self.pos = self.length

    def _start_tag(self, start):
        match = _TAG_NAME.match(self.source, start + 1)
        if not match:
            self.out.append("&lt;")
            self.pos = start + 1
            return
        name = match.group(0).lower()
        attrs, after = self._attributes(match.end())
        self.pos = after
        if name in DROP_CONTENT_TAGS:
            self._skip_element(name, after)
            return
        if name in ALLOWED_TAGS:
            self.out.append(self._render_start(name, attrs))
            if name in VOID_TAGS:
                return
            self.stack.append((name, True))
        elif name not in VOID_TAGS:
            # A self-closing slash does not turn a non-void element into a
            # void one; the element remains open.
            self.stack.append((name, False))

    def _end_tag(self, start):
        match = _TAG_NAME.match(self.source, start + 2)
        if not match:
            self.out.append("&lt;/")
            self.pos = start + 2
            return
        name = match.group(0).lower()
        gt = self.source.find(">", match.end())
        if gt < 0:
            self.pos = self.length
            return
        self.pos = gt + 1
        for index in range(len(self.stack) - 1, -1, -1):
            if self.stack[index][0] != name:
                continue
            self._close_to(index)
            return
        # An end tag that matches nothing is discarded, and scanning
        # continues with the following content.

    # -- helpers --------------------------------------------------------

    def _remainder(self):
        """Append the rest of the fragment and stop scanning."""
        self.out.append(self.source[self.pos:])
        self.pos = self.length

    def _close_to(self, index):
        while len(self.stack) > index:
            name, emitted = self.stack.pop()
            if emitted:
                self.out.append("</%s>" % name)

    def _attributes(self, start):
        attrs = []
        index = start
        while index < self.length:
            char = self.source[index]
            if char == ">":
                return attrs, index + 1
            if self.source.startswith("/>", index):
                return attrs, index + 2
            match = _ATTR.match(self.source, index)
            if not match:
                index += 1
                continue
            name = match.group(1).lower()
            for group in (2, 3, 4):
                value = match.group(group)
                if value is not None:
                    break
            attrs.append((name, value))
            index = match.end()
        return attrs, self.length

    def _skip_element(self, name, start):
        index = start
        while True:
            lt = self.source.find("</", index)
            if lt < 0:
                self.pos = self.length
                return
            match = _TAG_NAME.match(self.source, lt + 2)
            if match is None or match.group(0).lower() != name:
                index = lt + 2
                continue
            quote = None
            end = match.end()
            while end < self.length:
                char = self.source[end]
                if quote is not None:
                    if char == quote:
                        quote = None
                elif char in ("'", '"'):
                    quote = char
                elif char == ">":
                    self.pos = end + 1
                    return
                end += 1
            self.pos = self.length
            return

    def _render_start(self, name, attrs):
        pieces = ["<", name]
        for attr_name, value in allowed_attrs(attrs, unescape):
            if value is None:
                pieces.append(" " + attr_name)
            else:
                pieces.append(' %s="%s"' % (attr_name, escape(value, quote=True)))
        pieces.append(">")
        return "".join(pieces)
