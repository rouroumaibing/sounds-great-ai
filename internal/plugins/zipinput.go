package plugins

import (
	"archive/zip"
	"errors"
	"io"
)

// SeekableReader is the archive input contract: multipart.File (from
// r.FormFile) satisfies it, and it is what zip.NewReader needs.
type SeekableReader interface {
	io.Reader
	io.Seeker
	io.ReaderAt
}

// openZip reads a seekable stream into a *zip.Reader (size via seek-to-end).
func openZip(f SeekableReader) (*zip.Reader, func(), error) {
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, nil, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}
	if size <= 0 {
		return nil, nil, errors.New("empty archive")
	}
	if size > maxUncompressed {
		return nil, nil, errors.New("archive too large (max 64MiB)")
	}
	zr, err := zip.NewReader(f, size)
	if err != nil {
		return nil, nil, err
	}
	return zr, nil, nil
}
