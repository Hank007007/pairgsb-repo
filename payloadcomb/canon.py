"""Key normalisation helpers for decoded maps."""

import unicodedata


def normalize_key(key):
    """Return the comparison form used for map keys.

    Callers use this function both when a decoded map is built and when a
    schema's declared field names are looked up.  Two keys that compare equal
    after normalisation name the same field.
    """
    if not isinstance(key, str):
        key = str(key)
    return unicodedata.normalize("NFC", key).casefold()
