# Image intake policy

This document is the contract between the upload gateway and `mimesieve`.
The gateway calls `mimesieve.inspect(data, declared_type)` before anything is
written to object storage.  `inspect` returns a `Result` with:

| field | meaning |
|---|---|
| `format` | the container actually identified from the bytes, or `None` |
| `width`, `height` | the dimensions of the main image, or `None` |
| `allowed` | whether the object may be stored |
| `reason` | a rejection code from `mimesieve.errors.Reason`, or `None` |

`inspect` never raises for malformed input; every refusal is a `Result` with
`allowed=False`.  A container's dimensions are read from its structure only —
pixels are never decoded here.  The transcoding stage behind the gateway is
responsible for pixel data; this stage only decides admission.

## §1 Content type

The bytes are authoritative.  `declared_type` is the client-supplied
Content-Type and is advisory: it may be missing, wrong, or carry parameters
such as `; charset=binary`, which are normalised away before comparison.

The checks are applied in this order:

1. If a declared type is present and is not one of the four supported
   containers (§2), the upload is refused with `unsupported-type`.  This
   covers `image/svg+xml`, `image/heic`, `image/bmp` and non-image types such
   as `text/plain`.
2. The container is identified from the bytes, never from the declared type.
   If no container can be identified, the upload is refused with
   `unrecognized-format`.
3. If a declared type is present, is supported, and differs from the container
   identified from the bytes, the upload is refused with
   `content-type-mismatch`.

Magic-number checks must cover the whole signature, not a prefix of it, and
must be strong enough that an arbitrary file cannot pass by coincidence.

## §2 Supported containers

Exactly four containers are admitted:

* `image/png`
* `image/jpeg`
* `image/gif`
* `image/webp`

Every other media type is refused with `unsupported-type` if it is declared,
and any container whose bytes cannot be identified is refused with
`unrecognized-format`.

## §3 Structure walk

Each container is walked by its own module.  The walk must respect the
container's framing rules:

### §3.1 Truncation

Every length field in the container (PNG chunk length, JPEG segment length,
GIF sub-block size, WebP/RIFF chunk length) bounds a region of the file.  A
length that would extend past the end of the available bytes is a
`truncated` refusal.  The walker must not read past the end of the buffer and
must not allocate a buffer proportional to an unvalidated length field.

### §3.2 Exactly one dimension header

Each container must contain exactly one header that supplies the image
dimensions: PNG `IHDR`, a JPEG `SOF` frame header for the main image, the GIF
logical screen descriptor, or the WebP image chunk.  A second such header is
a `duplicate-header` refusal.  Later headers must not silently replace
earlier ones.

### §3.3 Main image, not embedded images

Container metadata can carry a complete nested image.  A JPEG EXIF (APP1)
segment may contain an embedded thumbnail with its own frame header; a PNG
may carry other ancillary payloads.  Dimensions reported by `inspect` are
those of the main image only.  Embedded images must never override the main
image's dimensions, and the main image's dimensions must never be taken from
inside another segment's payload.

Marker/segment scanning must honour each segment's declared length when
advancing: bytes inside a segment body are data, not markers.

### §3.4 End of file

The container terminator (PNG `IEND`, JPEG `EOI`, GIF trailer byte, the end
of the WebP RIFF payload) must be the last thing in the file.  Bytes after
the terminator are a `trailing-data` refusal, even if they are followed by a
second, well-formed terminator.  The tail check therefore compares the
position of the first terminator with the actual file length; checking only
the final bytes of the file is not sufficient.

## §4 Dimension limits

Let `w` and `h` be the dimensions reported by the walk.

* `w <= 100000` and `h <= 100000`, otherwise `dimension-limit`.
* `w * h <= 16777216`, otherwise `pixel-limit`.

Both comparisons are over the exact integer product.  No intermediate value
may wrap, saturate or otherwise change the outcome, regardless of how large
the dimensions are.

## §5 Reporting

For a refused upload, `width` and `height` are `None`; `format` is the
container identified from the bytes when one was identified, and `None`
otherwise.  For an admitted upload, `allowed` is `true`, `reason` is `None`,
and `width`/`height` are the main image's true dimensions.

## §6 Out of scope

`mimesieve` does not verify CRCs, decode pixel data, re-encode, inspect
colour profiles, or enforce file-size limits.  It reads only as much of each
container as is needed to reach and validate the dimension header and the
terminator.
