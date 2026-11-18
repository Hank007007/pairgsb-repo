package broker

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"time"
)

// Conn is the connection handle the rest of the broker works with. Reads are
// buffered so that a request can be decoded without a second allocation per
// byte, and writes go straight to the socket.
type Conn struct {
	*bufio.Reader
	conn *net.UnixConn
}

// NewConn wraps an accepted Unix connection.
func NewConn(c *net.UnixConn) *Conn {
	return &Conn{
		Reader: bufio.NewReaderSize(c, 64<<10),
		conn:   c,
	}
}

func (c *Conn) Read(p []byte) (int, error)  { return c.Reader.Read(p) }
func (c *Conn) Write(p []byte) (int, error) { return c.conn.Write(p) }
func (c *Conn) Close() error                { return c.conn.Close() }

func (c *Conn) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *Conn) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }

func (c *Conn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *Conn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *Conn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }

// Unix returns the underlying Unix connection.
func (c *Conn) Unix() *net.UnixConn { return c.conn }

// UnixListener accepts broker connections.
type UnixListener struct {
	ln *net.UnixListener
}

// Listen creates the broker socket.
func Listen(socketPath string) (*UnixListener, error) {
	addr := &net.UnixAddr{Name: socketPath, Net: "unix"}
	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		return nil, err
	}
	ln.SetUnlinkOnClose(true)
	return &UnixListener{ln: ln}, nil
}

// Accept waits for the next connection. The connection is wrapped in a
// buffered Conn before it is handed to the caller.
func (l *UnixListener) Accept() (net.Conn, error) {
	c, err := l.ln.AcceptUnix()
	if err != nil {
		return nil, err
	}
	return NewConn(c), nil
}

// Close closes the listening socket.
func (l *UnixListener) Close() error { return l.ln.Close() }

// Addr returns the socket address.
func (l *UnixListener) Addr() net.Addr { return l.ln.Addr() }

// Dial opens a client connection to a broker socket.
func Dial(socketPath string) (*Conn, error) {
	c, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return nil, err
	}
	return NewConn(c), nil
}

// SocketMode is the mode applied to a freshly created broker socket.
const SocketMode os.FileMode = 0o660

// ErrNoListener is returned when the socket path does not name a socket.
var ErrNoListener = errors.New("broker: no listener at the socket path")

var (
	_ net.Conn  = (*Conn)(nil)
	_ io.Reader = (*Conn)(nil)
)
