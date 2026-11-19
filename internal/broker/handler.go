package broker

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	"filesentry/internal/acl"
	"filesentry/internal/wire"
)

// DefaultMaxRead bounds the size of a file returned by a read request.
const DefaultMaxRead = 8 << 20

// Handler executes broker requests against a loaded policy.
type Handler struct {
	Policy *acl.Policy

	// Identity reports the uid of the process at the other end of a
	// connection. When nil the platform credential source is used.
	Identity func(net.Conn) (int, error)

	// MaxRead bounds read responses. Zero means DefaultMaxRead.
	MaxRead int64
}

// Handle serves exactly one request on c and writes one response.
func (h *Handler) Handle(c net.Conn) {
	defer c.Close()

	resp := h.serve(c)
	if err := wire.EncodeResponse(c, resp); err != nil {
		return
	}
}

func (h *Handler) serve(c net.Conn) *wire.Response {
	if c == nil {
		return reject(wire.ReasonInvalidRequest, errors.New("broker: no connection"))
	}

	req, err := wire.DecodeRequest(bufio.NewReader(c))
	if err != nil {
		return reject(wire.ReasonInvalidRequest, fmt.Errorf("decode request: %w", err))
	}

	if req.Version != wire.Version {
		return reject(wire.ReasonInvalidRequest, fmt.Errorf("unsupported version %d", req.Version))
	}

	identity := h.Identity
	if identity == nil {
		identity = peerIdentity
	}
	uid, err := identity(c)
	if err != nil {
		return reject(wire.ReasonUIDUnavailable, err)
	}

	target, err := acl.NormalizePath(req.Path)
	if err != nil {
		return reject(wire.ReasonInvalidRequest, err)
	}

	if req.Op == wire.OpChown {
		return h.Chown(uid, target, req)
	}

	switch req.Op {
	case wire.OpRead, wire.OpWrite:
		rule, pattern, ok := h.Policy.SelectRule(uid, target)
		if !ok || rule.Denies() || !rule.Allows(req.Op) {
			return reject(wire.ReasonDenied, fmt.Errorf("uid %d may not %s %s", uid, req.Op, target))
		}

		flag := os.O_RDONLY
		var perm os.FileMode
		if req.Op == wire.OpWrite {
			flag = os.O_WRONLY | os.O_CREATE
			perm = os.FileMode(req.Mode) & 0o777
		}

		opened, err := resolveFile(target, acl.PatternRoot(pattern), flag, perm)
		if err != nil {
			return reject(reasonFor(err), err)
		}
		defer opened.file.Close()

		if req.Op == wire.OpRead {
			if err := requireRegular(opened.file); err != nil {
				return reject(reasonFor(err), err)
			}
			return h.read(opened.file)
		}
		return h.write(opened.file, req)

	case wire.OpList:
		rule, pattern, ok := h.Policy.SelectRule(uid, target)
		if !ok || rule.Denies() || !rule.Allows(req.Op) {
			return reject(wire.ReasonDenied, fmt.Errorf("uid %d may not %s %s", uid, req.Op, target))
		}

		opened, err := resolveDirectory(target, acl.PatternRoot(pattern))
		if err != nil {
			return reject(reasonFor(err), err)
		}
		defer opened.file.Close()

		if fi, statErr := opened.file.Stat(); statErr != nil {
			return reject(wire.ReasonIOError, statErr)
		} else if !fi.IsDir() {
			return reject(reasonFor(errNotDirectory), errNotDirectory)
		}
		return h.list(opened.file)

	default:
		return reject(wire.ReasonInvalidRequest, fmt.Errorf("unknown op %q", req.Op))
	}
}

// requireRegular verifies that an opened read target is a regular file.
func requireRegular(f *os.File) error {
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if fi.IsDir() || !fi.Mode().IsRegular() {
		return errNotRegular
	}
	return nil
}

func (h *Handler) read(f *os.File) *wire.Response {
	limit := h.MaxRead
	if limit <= 0 {
		limit = DefaultMaxRead
	}

	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return reject(wire.ReasonIOError, err)
	}
	if int64(len(data)) > limit {
		return reject(wire.ReasonTooLarge, fmt.Errorf("file is larger than %d bytes", limit))
	}
	return &wire.Response{Version: wire.Version, OK: true, Data: data}
}

func (h *Handler) write(f *os.File, req *wire.Request) *wire.Response {
	if req.Mode != 0 {
		if err := f.Chmod(os.FileMode(req.Mode) & 0o777); err != nil {
			return reject(wire.ReasonIOError, err)
		}
	}

	if err := f.Truncate(0); err != nil {
		return reject(wire.ReasonIOError, err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return reject(wire.ReasonIOError, err)
	}
	if _, err := f.Write(req.Data); err != nil {
		return reject(wire.ReasonIOError, err)
	}
	return &wire.Response{Version: wire.Version, OK: true}
}

func (h *Handler) list(f *os.File) *wire.Response {
	names, err := f.Readdirnames(-1)
	if err != nil {
		return reject(wire.ReasonIOError, err)
	}

	entries := make([]wire.Entry, 0, len(names))
	for _, name := range names {
		entry, err := describe(f.Name(), name)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	sortEntries(entries)

	return &wire.Response{Version: wire.Version, OK: true, Entries: entries}
}

func reject(reason string, err error) *wire.Response {
	resp := &wire.Response{Version: wire.Version, Reason: reason}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp
}

var (
	errPathEscape   = errors.New("broker: resolved path leaves the authorised root")
	errNotRegular   = errors.New("broker: target is not a regular file")
	errNotDirectory = errors.New("broker: target is not a directory")
)

func reasonFor(err error) string {
	switch {
	case errors.Is(err, errPathEscape):
		return wire.ReasonPathEscape
	case errors.Is(err, errNotRegular), errors.Is(err, errNotDirectory):
		return wire.ReasonInvalidRequest
	case errors.Is(err, os.ErrNotExist):
		return wire.ReasonNotFound
	default:
		return wire.ReasonIOError
	}
}
