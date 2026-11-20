# mimesieve

Upload-intake gate for images.  The gateway calls

```python
from mimesieve import inspect

result = inspect(data, declared_type)
if not result.allowed:
    ...  # result.reason carries the rejection code
```

`inspect` identifies the container from the file structure, reads the main
image dimensions, and applies the admission policy.  Pixels are never
decoded; a separate transcoding service handles that downstream.

The policy is specified in [`docs/image-intake-policy.md`](docs/image-intake-policy.md);
the module layout and data flow are described in
[`docs/design.md`](docs/design.md).

## Layout

```
mimesieve/
  sniff.py     container identification from the leading bytes
  png.py       PNG chunk walk
  jpeg.py      JPEG marker walk
  gif.py       GIF logical-screen walk
  webp.py      WebP RIFF walk
  verify.py    inspect(): format -> dimensions -> tail -> policy
  policy.py    supported containers and limits
  errors.py    rejection codes
tools/intake.py           report over testdata/images/
testdata/images/          sample uploads (a .ctype sidecar holds the declared type)
tests/                    pytest suite
```

## Running

```
python -m pytest -q -p no:cacheprovider
python tools/intake.py
```
