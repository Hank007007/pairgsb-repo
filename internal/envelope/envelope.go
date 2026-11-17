// Package envelope implements object-level envelope encryption.
package envelope

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"

	"kekring/internal/keyring"
	"kekring/internal/wrap"
)

const (
	// NonceSize is the size in bytes of an AES-GCM nonce.
	NonceSize = 12
	// TagSize is the size in bytes of an AES-GCM authentication tag.
	TagSize = 16
)

// Header carries the sealed data key and the parameters needed to open it.
type Header struct {
	KEKVersion uint32
	Algorithm  string
	DEKNonce   []byte
	SealedDEK  []byte
}

// Envelope is a sealed object: a header, a body nonce and the body ciphertext.
type Envelope struct {
	Header    Header
	BodyNonce []byte
	Body      []byte
}

var (
	// ErrMalformed identifies bytes that are not a well-formed envelope.
	ErrMalformed = errors.New("malformed envelope")
	// ErrAuthentication identifies an envelope that failed authentication.
	ErrAuthentication = errors.New("envelope failed authentication")
)

// Seal encrypts plaintext for objectID under the keyring's write key.
func Seal(kr *keyring.Keyring, objectID string, plaintext []byte) ([]byte, error) {
	key, err := kr.SelectForWrite()
	if err != nil {
		return nil, err
	}

	dek := make([]byte, wrap.DEKSize)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("generate data key: %w", err)
	}

	sealed, err := wrap.WrapDEK(key.Material, key.Version, objectID, wrap.Algorithm, dek)
	if err != nil {
		return nil, err
	}

	bodyNonce := make([]byte, NonceSize)
	if _, err := rand.Read(bodyNonce); err != nil {
		return nil, fmt.Errorf("generate body nonce: %w", err)
	}

	aead, err := newAEAD(dek)
	if err != nil {
		return nil, err
	}
	body := aead.Seal(nil, bodyNonce, plaintext, nil)

	env := &Envelope{
		Header: Header{
			KEKVersion: key.Version,
			Algorithm:  sealed.Algorithm,
			DEKNonce:   sealed.Nonce,
			SealedDEK:  sealed.Ciphertext,
		},
		BodyNonce: bodyNonce,
		Body:      body,
	}
	return env.Marshal()
}

// Open decrypts blob for objectID.
func Open(kr *keyring.Keyring, objectID string, blob []byte) ([]byte, error) {
	env, err := Parse(blob)
	if err != nil {
		return nil, err
	}

	key, err := kr.SelectForRead(env.Header.KEKVersion)
	if err != nil {
		return nil, err
	}

	dek, err := wrap.UnwrapDEK(
		key.Material,
		env.Header.KEKVersion,
		objectID,
		env.Header.Algorithm,
		env.Header.DEKNonce,
		env.Header.SealedDEK,
	)
	if err != nil {
		return nil, ErrAuthentication
	}

	aead, err := newAEAD(dek)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, env.BodyNonce, env.Body, nil)
	if err != nil {
		return nil, ErrAuthentication
	}
	return plaintext, nil
}

// Rewrap re-seals the data key of blob under the keyring's write key. The body
// nonce and body ciphertext are left untouched.
func Rewrap(kr *keyring.Keyring, objectID string, blob []byte) ([]byte, error) {
	env, err := Parse(blob)
	if err != nil {
		return nil, err
	}

	readKey, err := kr.SelectForRead(env.Header.KEKVersion)
	if err != nil {
		return nil, err
	}

	dek, err := wrap.UnwrapDEK(
		readKey.Material,
		env.Header.KEKVersion,
		objectID,
		env.Header.Algorithm,
		env.Header.DEKNonce,
		env.Header.SealedDEK,
	)
	if err != nil {
		return nil, ErrAuthentication
	}

	writeKey, err := kr.SelectForWrite()
	if err != nil {
		return nil, err
	}

	sealed, err := wrap.WrapDEK(writeKey.Material, writeKey.Version, objectID, env.Header.Algorithm, dek)
	if err != nil {
		return nil, err
	}

	env.Header.KEKVersion = writeKey.Version
	env.Header.Algorithm = sealed.Algorithm
	env.Header.DEKNonce = sealed.Nonce
	env.Header.SealedDEK = sealed.Ciphertext
	return env.Marshal()
}

// Marshal serialises e.
func (e *Envelope) Marshal() ([]byte, error) {
	if len(e.Header.DEKNonce) != NonceSize {
		return nil, fmt.Errorf("%w: data key nonce is %d bytes", ErrMalformed, len(e.Header.DEKNonce))
	}
	if len(e.Header.SealedDEK) != wrap.DEKSize+TagSize {
		return nil, fmt.Errorf("%w: sealed data key is %d bytes", ErrMalformed, len(e.Header.SealedDEK))
	}
	if len(e.Header.Algorithm) == 0 || len(e.Header.Algorithm) > 255 {
		return nil, fmt.Errorf("%w: algorithm name is invalid", ErrMalformed)
	}
	if len(e.BodyNonce) != NonceSize {
		return nil, fmt.Errorf("%w: body nonce is %d bytes", ErrMalformed, len(e.BodyNonce))
	}
	if len(e.Body) < TagSize {
		return nil, fmt.Errorf("%w: body is too short", ErrMalformed)
	}

	out := make([]byte, 0, len(e.Header.DEKNonce)+len(e.Header.SealedDEK)+4+1+len(e.Header.Algorithm)+len(e.BodyNonce)+len(e.Body))
	out = append(out, e.Header.DEKNonce...)
	out = append(out, e.Header.SealedDEK...)
	out = binary.BigEndian.AppendUint32(out, e.Header.KEKVersion)
	out = append(out, byte(len(e.Header.Algorithm)))
	out = append(out, e.Header.Algorithm...)
	out = append(out, e.BodyNonce...)
	out = append(out, e.Body...)
	return out, nil
}

// Parse decodes an envelope.
func Parse(blob []byte) (*Envelope, error) {
	const fixed = NonceSize + wrap.DEKSize + TagSize + 4 + 1
	if len(blob) < fixed+NonceSize+TagSize {
		return nil, fmt.Errorf("%w: only %d bytes", ErrMalformed, len(blob))
	}

	off := 0
	dekNonce := append([]byte(nil), blob[off:off+NonceSize]...)
	off += NonceSize
	sealedDEK := append([]byte(nil), blob[off:off+wrap.DEKSize+TagSize]...)
	off += wrap.DEKSize + TagSize
	version := binary.BigEndian.Uint32(blob[off : off+4])
	off += 4
	algLen := int(blob[off])
	off++

	if len(blob) < off+algLen+NonceSize+TagSize {
		return nil, fmt.Errorf("%w: truncated header", ErrMalformed)
	}
	algorithm := string(blob[off : off+algLen])
	off += algLen
	if algorithm == "" {
		return nil, fmt.Errorf("%w: empty algorithm name", ErrMalformed)
	}

	bodyNonce := append([]byte(nil), blob[off:off+NonceSize]...)
	off += NonceSize
	body := append([]byte(nil), blob[off:]...)

	return &Envelope{
		Header: Header{
			KEKVersion: version,
			Algorithm:  algorithm,
			DEKNonce:   dekNonce,
			SealedDEK:  sealedDEK,
		},
		BodyNonce: bodyNonce,
		Body:      body,
	}, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialise block cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialise gcm: %w", err)
	}
	return aead, nil
}
