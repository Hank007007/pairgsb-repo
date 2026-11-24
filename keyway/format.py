"""Wire constants shared by the blob codec and the AEAD layer.

The header layout is described in ``docs/blob-format.md`` §1.1.
"""

MAGIC = b"KWTBLOB\x00"
FORMAT_VERSION = 1

# magic + format version + key version + key-name length
PREFIX_LEN = 8 + 1 + 1 + 1

NONCE_LEN = 12
TAG_LEN = 16

KEY_LEN = 32
MAX_KEY_NAME_LEN = 255
