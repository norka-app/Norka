package diagnostics

import (
	"bytes"
	"io"
	"os"
)

const (
	// MaxLogBytes is the cap on the journal copied into the archive.
	MaxLogBytes = 2 << 20
	// MaxLogLines is the cap on how many trailing lines are kept.
	MaxLogLines = 2000
)

// LogPath is norka.log next to the config file.
func LogPath(configPath string) string {
	return beside(configPath, "norka.log")
}

// Tail keeps the trailing part of data, limited by bytes and then by lines.
func Tail(data []byte, maxBytes, maxLines int) []byte {
	if len(data) == 0 {
		return []byte{}
	}
	if maxBytes < 1 {
		maxBytes = MaxLogBytes
	}
	partial := false
	if len(data) > maxBytes {
		data = data[len(data)-maxBytes:]
		partial = true
	}
	if partial {
		if i := bytes.IndexByte(data, '\n'); i >= 0 && i+1 < len(data) {
			data = data[i+1:]
		}
	}
	if maxLines > 0 {
		lines := bytes.Split(data, []byte{'\n'})
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
			data = bytes.Join(lines, []byte{'\n'})
		}
	}
	if len(data) > maxBytes {
		data = data[len(data)-maxBytes:]
	}
	return data
}

// ReadLogTail reads at most MaxLogBytes from the end of path.
// A missing file is an empty log, not an error.
func ReadLogTail(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	start := int64(0)
	if info.Size() > int64(MaxLogBytes) {
		start = info.Size() - int64(MaxLogBytes)
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(MaxLogBytes)))
	if err != nil {
		return nil, err
	}
	if start > 0 {
		// Drop a line that was cut in half by the seek. Keep the window when
		// the only newline is the final byte, so the last entry is not lost.
		if i := bytes.IndexByte(data, '\n'); i >= 0 && i+1 < len(data) {
			data = data[i+1:]
		}
	}
	return Tail(data, MaxLogBytes, MaxLogLines), nil
}
