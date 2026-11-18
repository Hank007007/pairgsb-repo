package wire

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
)

// maxMessageBytes bounds a single encoded message so that a peer cannot make
// the broker allocate without limit.
const maxMessageBytes = 32 << 20

// DecodeRequest reads one newline-terminated request.
func DecodeRequest(r *bufio.Reader) (*Request, error) {
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}
	req := new(Request)
	if err := json.Unmarshal(line, req); err != nil {
		return nil, err
	}
	return req, nil
}

// EncodeRequest writes one newline-terminated request.
func EncodeRequest(w io.Writer, req *Request) error {
	return writeLine(w, req)
}

// DecodeResponse reads one newline-terminated response.
func DecodeResponse(r *bufio.Reader) (*Response, error) {
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}
	resp := new(Response)
	if err := json.Unmarshal(line, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// EncodeResponse writes one newline-terminated response.
func EncodeResponse(w io.Writer, resp *Response) error {
	return writeLine(w, resp)
}

func readLine(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && len(line) > 0 {
			return line, nil
		}
		return nil, err
	}
	if len(line) > maxMessageBytes {
		return nil, errors.New("message exceeds the maximum size")
	}
	return line, nil
}

func writeLine(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(b)+1 > maxMessageBytes {
		return errors.New("message exceeds the maximum size")
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}
