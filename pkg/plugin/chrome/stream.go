package chrome

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	cdprotoio "github.com/chromedp/cdproto/io"
)

// Interface guards.
var _ io.Reader = (*streamReader)(nil)

// Credits: https://raw.githubusercontent.com/mafredri/cdp/3c5eab7ffc5cbee667b0a813ce470ac423792811/protocol/io/stream_reader.go.
type streamReader struct {
	ctx    context.Context
	handle cdprotoio.StreamHandle
	r      io.Reader
	pos    int
	eof    bool
}

// NewStreamReader creates a new stream reader.
func NewStreamReader(ctx context.Context, handle cdprotoio.StreamHandle) io.ReadCloser {
	return &streamReader{ctx: ctx, handle: handle}
}

// Read a chunk of the stream.
func (reader *streamReader) Read(p []byte) (int, error) {
	if reader.r != nil {
		// Continue reading from buffer.
		return reader.read(p)
	}

	if reader.eof {
		return 0, io.EOF
	}

	if len(p) == 0 {
		return 0, nil
	}

	// Chromium might have an off-by-one when deciding the maximum size (at
	// least for base64 encoded data), usually it will overflow. We subtract
	// one to make sure it fits into p.
	size := max(len(p)-1,
		// Safety-check to avoid crashing Chrome (e.g. via SetSize(-1)).
		1)

	reply, err := reader.next(reader.pos, size)
	if err != nil {
		return 0, err
	}

	reader.eof = reply.EOF

	switch {
	case reply.Base64encoded:
		b := []byte(reply.Data)
		size := base64.StdEncoding.DecodedLen(len(b))

		// Safety-check for fast-path to avoid panics.
		if len(p) >= size {
			n, err := base64.StdEncoding.Decode(p, b)
			reader.pos += n

			return n, err
		}

		reader.r = base64.NewDecoder(base64.StdEncoding, bytes.NewReader(b))
	default:
		reader.r = strings.NewReader(reply.Data)
	}

	return reader.read(p)
}

// Close closes the stream, discard any temporary backing storage.
func (reader *streamReader) Close() error {
	err := cdprotoio.Close(reader.handle).Do(reader.ctx)
	if err == nil {
		return nil
	}

	return fmt.Errorf("close Chromium stream: %w", err)
}

func (reader *streamReader) next(pos, size int) (cdprotoio.ReadReturns, error) {
	params := cdprotoio.
		Read(reader.handle).
		WithOffset(int64(pos)).
		WithSize(int64(size))

	var res cdprotoio.ReadReturns
	err := cdp.Execute(reader.ctx, cdprotoio.CommandRead, params, &res)

	if err == nil {
		return res, nil
	}

	return res, fmt.Errorf("execute IO.read command: %w", err)
}

func (reader *streamReader) read(p []byte) (int, error) {
	n, err := reader.r.Read(p)
	reader.pos += n

	if !reader.eof && err == io.EOF {
		reader.r = nil
		err = nil
	}

	return n, err
}
