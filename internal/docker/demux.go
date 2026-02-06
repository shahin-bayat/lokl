package docker

import (
	"encoding/binary"
	"io"
)

const streamHeaderSize = 8

// demuxReader strips Docker's 8-byte multiplexed stream headers (used when
// TTY is disabled) so consumers get raw log output.
type demuxReader struct {
	src       io.ReadCloser
	remaining int
}

func newDemuxReader(src io.ReadCloser) *demuxReader {
	return &demuxReader{src: src}
}

func (r *demuxReader) Read(p []byte) (int, error) {
	for r.remaining == 0 {
		var header [streamHeaderSize]byte
		if _, err := io.ReadFull(r.src, header[:]); err != nil {
			return 0, err
		}
		r.remaining = int(binary.BigEndian.Uint32(header[4:]))
	}

	toRead := len(p)
	if toRead > r.remaining {
		toRead = r.remaining
	}
	n, err := r.src.Read(p[:toRead])
	r.remaining -= n
	return n, err
}

func (r *demuxReader) Close() error {
	return r.src.Close()
}
