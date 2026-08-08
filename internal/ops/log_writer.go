package ops

import (
	"bytes"
	"io"
	"time"
)

// LogWriter is an io.Writer that tees log output to both an underlying writer
// (typically os.Stderr) and a LogBuffer. It parses each log line to extract
// the timestamp and message, then stores it in the ring buffer.
type LogWriter struct {
	dest  io.Writer
	buf   *LogBuffer
	level string
	// leftover holds an incomplete line across Write calls
	leftover []byte
}

// NewLogWriter creates a LogWriter that writes to dest and records entries in buf.
// The level field is used for all entries (default "INFO").
func NewLogWriter(dest io.Writer, buf *LogBuffer) *LogWriter {
	return &LogWriter{dest: dest, buf: buf, level: "INFO"}
}

// NewLogWriterWithLevel creates a LogWriter with a custom default level.
func NewLogWriterWithLevel(dest io.Writer, buf *LogBuffer, level string) *LogWriter {
	return &LogWriter{dest: dest, buf: buf, level: level}
}

func (w *LogWriter) Write(p []byte) (int, error) {
	// Always pass through to the real writer first
	n, err := w.dest.Write(p)
	if err != nil {
		return n, err
	}

	// Process complete lines. The standard log package writes one line per call
	// (terminated by \n), but we handle partial lines defensively.
	data := append(w.leftover, p...)
	w.leftover = nil

	for {
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			// No complete line; save remainder
			w.leftover = make([]byte, len(data))
			copy(w.leftover, data)
			break
		}
		line := data[:idx]
		data = data[idx+1:]
		w.parseAndAdd(line)
	}

	return n, nil
}

// parseAndAdd parses a log line and adds it to the LogBuffer.
// Standard log format: "2006/01/02 15:04:05 message"
// We extract the message portion; if parsing fails, store the raw line.
func (w *LogWriter) parseAndAdd(line []byte) {
	if len(line) == 0 {
		return
	}
	msg := string(line)

	// Try to strip the standard log prefix "2006/01/02 15:04:05 "
	if len(msg) > 20 && msg[4] == '/' && msg[7] == '/' && msg[10] == ' ' && msg[13] == ':' {
		msg = msg[20:]
	}

	w.buf.Add(w.level, msg)
}

// Flush forces any buffered partial line into the LogBuffer.
func (w *LogWriter) Flush() {
	if len(w.leftover) > 0 {
		w.parseAndAdd(w.leftover)
		w.leftover = nil
	}
}

// Now returns the current time. Exposed for testing.
var now = time.Now
