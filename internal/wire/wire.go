package wire

const Version = 1

// Operation names exchanged over the socket.
const (
	OpRead  = "read"
	OpWrite = "write"
	OpList  = "list"
	OpChown = "chown"
)

// Reason codes carried by a refused response.
const (
	ReasonDenied         = "denied"
	ReasonPathEscape     = "path_escape"
	ReasonNotFound       = "not_found"
	ReasonInvalidRequest = "invalid_request"
	ReasonTooLarge       = "too_large"
	ReasonUIDUnavailable = "uid_unavailable"
	ReasonIOError        = "io_error"
)

// Request is the single message a client sends over a connection.
type Request struct {
	Version int    `json:"version"`
	Op      string `json:"op"`
	Path    string `json:"path"`
	Data    []byte `json:"data,omitempty"`
	Mode    uint32 `json:"mode,omitempty"`
	UID     int    `json:"uid,omitempty"`
	GID     int    `json:"gid,omitempty"`
}

// Entry describes one directory entry returned by a list request.
type Entry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
	Mode  uint32 `json:"mode"`
	UID   int    `json:"uid"`
	GID   int    `json:"gid"`
}

// Response is the single message the broker sends back.
type Response struct {
	Version int     `json:"version"`
	OK      bool    `json:"ok"`
	Data    []byte  `json:"data,omitempty"`
	Entries []Entry `json:"entries,omitempty"`
	Reason  string  `json:"reason,omitempty"`
	Error   string  `json:"error,omitempty"`
}

// OK reports whether the response carries a completed operation.
func (r *Response) OKStatus() bool { return r.OK }
