package fcgx

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	FCGI_HEADER_LEN     = 8
	fcgiVersion1        = 1
	fcgiBeginRequest    = 1
	fcgiAbortRequest    = 2
	fcgiEndRequest      = 3
	fcgiParams          = 4
	fcgiStdin           = 5
	fcgiStdout          = 6
	fcgiStderr          = 7
	fcgiResponder       = 1
	fcgiRequestComplete = 0
	maxWrite            = 65500
	maxPad              = 255
)

type header struct {
	Version       uint8
	Type          uint8
	RequestID     uint16
	ContentLength uint16
	PaddingLength uint8
	Reserved      uint8
}

type Client struct {
	conn   net.Conn
	mu     sync.Mutex
	reqID  uint16
	closed bool
	buf    bytes.Buffer
}

// Helper to write a FastCGI record
func (c *Client) writeRecord(recType uint8, content []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.buf.Reset()
	contentLen := len(content)
	padLen := uint8((8 - (contentLen % 8)) % 8)

	h := header{
		Version:       fcgiVersion1,
		Type:          recType,
		RequestID:     c.reqID,
		ContentLength: uint16(contentLen),
		PaddingLength: padLen,
	}

	if err := binary.Write(&c.buf, binary.BigEndian, h); err != nil {
		return fmt.Errorf("writing record header: %w", err)
	}

	if contentLen > 0 {
		c.buf.Write(content)
	}

	if padLen > 0 {
		c.buf.Write(make([]byte, padLen))
	}

	_, err := c.conn.Write(c.buf.Bytes())
	if err != nil {
		if err, ok := err.(net.Error); ok && err.Timeout() {
			return fmt.Errorf("timeout while writing record: %w", err)
		}
		return fmt.Errorf("writing record: %w", err)
	}
	return nil
}

func (c *Client) writeBeginRequest(role uint16, flags uint8) error {
	b := [8]byte{byte(role >> 8), byte(role), flags}
	return c.writeRecord(fcgiBeginRequest, b[:])
}

func encodePair(w *bytes.Buffer, k, v string) {
	writeSize := func(size int) {
		if size < 128 {
			w.WriteByte(byte(size))
		} else {
			sz := uint32(size) | (1 << 31)
			_ = binary.Write(w, binary.BigEndian, sz)
		}
	}
	writeSize(len(k))
	writeSize(len(v))
	w.WriteString(k)
	w.WriteString(v)
}

func (c *Client) writePairs(recType uint8, pairs map[string]string) error {
	w := &bytes.Buffer{}
	for k, v := range pairs {
		encodePair(w, k, v)
	}
	return c.writeRecord(recType, w.Bytes())
}

func (c *Client) DoRequest(ctx context.Context, params map[string]string, body io.Reader) (*http.Response, error) {
	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context error: %w", err)
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, errors.New("client closed")
	}
	c.mu.Unlock()

	// Set deadline from context
	deadline, ok := ctx.Deadline()
	if ok {
		if err := c.conn.SetDeadline(deadline); err != nil {
			return nil, fmt.Errorf("setting deadline: %w", err)
		}
		// Reset deadline after request
		defer c.conn.SetDeadline(time.Time{})
	}

	// BEGIN_REQUEST record
	if err := c.writeBeginRequest(uint16(fcgiResponder), 0); err != nil {
		return nil, fmt.Errorf("writing begin request: %w", err)
	}

	// Check context after each major operation
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context error: %w", err)
	}

	// PARAMS records
	if err := c.writePairs(fcgiParams, params); err != nil {
		return nil, fmt.Errorf("writing params: %w", err)
	}

	// Send terminating empty PARAMS record
	if err := c.writeRecord(fcgiParams, nil); err != nil {
		return nil, fmt.Errorf("writing empty params: %w", err)
	}

	// Check context after params
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context error: %w", err)
	}

	// STDIN records
	if body != nil {
		bodyBuf := &bytes.Buffer{}
		if _, err := io.Copy(bodyBuf, body); err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		data := bodyBuf.Bytes()

		total := len(data)
		offset := 0
		for offset < total {
			// Check context before each chunk
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("context error: %w", err)
			}

			chunkSize := total - offset
			if chunkSize > maxWrite {
				chunkSize = maxWrite
			}
			chunk := data[offset : offset+chunkSize]
			if err := c.writeRecord(fcgiStdin, chunk); err != nil {
				return nil, fmt.Errorf("writing stdin chunk: %w", err)
			}
			offset += chunkSize
		}
	}

	// Always send terminating empty STDIN record
	if err := c.writeRecord(fcgiStdin, nil); err != nil {
		return nil, fmt.Errorf("writing empty stdin: %w", err)
	}

	// Read response
	respBuf := &bytes.Buffer{}
	endRequestReceived := false

	for {
		// Check context before each read
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context error: %w", err)
		}

		h := header{}
		if err := binary.Read(c.conn, binary.BigEndian, &h); err != nil {
			if err == io.EOF {
				if respBuf.Len() > 0 && endRequestReceived {
					break
				}
				return nil, fmt.Errorf("unexpected EOF while reading header")
			}
			if err, ok := err.(net.Error); ok && err.Timeout() {
				return nil, fmt.Errorf("timeout while reading header: %w", err)
			}
			return nil, fmt.Errorf("reading response header: %w", err)
		}

		if h.Type == fcgiStdout || h.Type == fcgiStderr {
			b := make([]byte, h.ContentLength)
			if _, err := io.ReadFull(c.conn, b); err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					return nil, fmt.Errorf("timeout while reading response body: %w", err)
				}
				return nil, fmt.Errorf("reading response body: %w", err)
			}
			respBuf.Write(b)

			if h.PaddingLength > 0 {
				if _, err := io.CopyN(io.Discard, c.conn, int64(h.PaddingLength)); err != nil {
					if err, ok := err.(net.Error); ok && err.Timeout() {
						return nil, fmt.Errorf("timeout while reading padding: %w", err)
					}
					return nil, fmt.Errorf("reading padding: %w", err)
				}
			}
		} else if h.Type == fcgiEndRequest {
			endRequestReceived = true
			if h.ContentLength > 0 {
				if _, err := io.CopyN(io.Discard, c.conn, int64(h.ContentLength)); err != nil {
					if err, ok := err.(net.Error); ok && err.Timeout() {
						return nil, fmt.Errorf("timeout while reading end request body: %w", err)
					}
					return nil, fmt.Errorf("reading end request body: %w", err)
				}
			}
			if h.PaddingLength > 0 {
				if _, err := io.CopyN(io.Discard, c.conn, int64(h.PaddingLength)); err != nil {
					if err, ok := err.(net.Error); ok && err.Timeout() {
						return nil, fmt.Errorf("timeout while reading end request padding: %w", err)
					}
					return nil, fmt.Errorf("reading end request padding: %w", err)
				}
			}
			if respBuf.Len() > 0 {
				break
			}
		}
	}

	resp, err := parseHTTPResponse(respBuf)
	if err != nil {
		return nil, fmt.Errorf("parsing HTTP response: %w", err)
	}
	return resp, nil
}

func parseHTTPResponse(buf *bytes.Buffer) (*http.Response, error) {
	reader := bufio.NewReader(buf)
	tp := textproto.NewReader(reader)

	line, err := tp.ReadLine()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	// If missing HTTP headers, fallback to plain-text body
	if !strings.HasPrefix(line, "HTTP/") && !strings.HasPrefix(line, "Status:") {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     make(http.Header),
			Body:       io.NopCloser(io.MultiReader(strings.NewReader(line+"\n"), reader)),
		}, nil
	}
	// Handle status lines without protocol, e.g., "Status: 200 OK"
	if strings.HasPrefix(line, "Status: ") {
		line = "HTTP/1.1 " + strings.TrimPrefix(line, "Status: ")
	}
	if i := strings.IndexByte(line, ' '); i == -1 {
		return nil, fmt.Errorf("malformed HTTP response %q", line)
	} else {
		resp := new(http.Response)
		resp.Proto = line[:i]
		resp.Status = strings.TrimLeft(line[i+1:], " ")

		statusCode := resp.Status
		if i := strings.IndexByte(resp.Status, ' '); i != -1 {
			statusCode = resp.Status[:i]
		}
		if len(statusCode) != 3 {
			return nil, fmt.Errorf("malformed HTTP status code %q", statusCode)
		}
		resp.StatusCode, err = strconv.Atoi(statusCode)
		if err != nil || resp.StatusCode < 0 {
			return nil, fmt.Errorf("invalid HTTP status code %q", statusCode)
		}

		var ok bool
		if resp.ProtoMajor, resp.ProtoMinor, ok = http.ParseHTTPVersion(resp.Proto); !ok {
			return nil, fmt.Errorf("malformed HTTP version %q", resp.Proto)
		}

		// Headers
		mimeHeader, err := tp.ReadMIMEHeader()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}

		resp.Header = http.Header(mimeHeader)
		resp.TransferEncoding = resp.Header["Transfer-Encoding"]
		resp.ContentLength, _ = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)

		if chunked(resp.TransferEncoding) {
			resp.Body = io.NopCloser(httputil.NewChunkedReader(reader))
		} else {
			resp.Body = io.NopCloser(reader)
		}

		return resp, nil
	}
}

func (c *Client) Get(ctx context.Context, params map[string]string) (*http.Response, error) {
	params["REQUEST_METHOD"] = "GET"
	params["CONTENT_LENGTH"] = "0"
	return c.DoRequest(ctx, params, nil)
}

func (c *Client) Post(ctx context.Context, params map[string]string, body io.Reader, contentLength int) (*http.Response, error) {
	params["REQUEST_METHOD"] = "POST"
	params["CONTENT_LENGTH"] = strconv.Itoa(contentLength)
	if _, ok := params["CONTENT_TYPE"]; !ok {
		params["CONTENT_TYPE"] = "application/x-www-form-urlencoded"
	}

	// Ensure we have a valid body reader
	if body == nil {
		body = bytes.NewReader(nil)
	}

	// If body is a string reader, ensure it's properly formatted
	if sr, ok := body.(*strings.Reader); ok {
		buf := make([]byte, sr.Len())
		sr.Read(buf)
		body = bytes.NewReader(buf)
	}

	return c.DoRequest(ctx, params, body)
}

func chunked(te []string) bool {
	return len(te) > 0 && te[0] == "chunked"
}

// Dial establishes a connection to the FastCGI server at the specified network address.
func Dial(network, address string) (*Client, error) {
	// Use a reasonable default timeout for initial connection
	dialer := net.Dialer{
		Timeout: 5 * time.Second,
	}
	conn, err := dialer.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, reqID: 1}, nil
}

// DialContext establishes a connection to the FastCGI server at the specified network address
// with the given context.
func DialContext(ctx context.Context, network, address string) (*Client, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, reqID: 1}, nil
}

// Close closes the FastCGI connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return c.conn.Close()
}
