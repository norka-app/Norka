package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
)

// Call sends one request and reads one response.
// A missing socket or pipe is ErrNotRunning.
func Call(ctx context.Context, address, token string, req Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	req.Token = token
	conn, err := dial(ctx, address)
	if err != nil {
		if isNotRunning(err) {
			return Response{}, ErrNotRunning
		}
		return Response{}, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if err := writeJSON(conn, req); err != nil {
		if isNotRunning(err) {
			return Response{}, ErrNotRunning
		}
		return Response{}, err
	}
	var resp Response
	if err := readJSON(conn, &resp); err != nil {
		if isNotRunning(err) {
			return Response{}, ErrNotRunning
		}
		return Response{}, err
	}
	resp.Running = true
	return resp, nil
}

// Follower reads a subscribe connection: one snapshot, then events.
type Follower struct {
	conn net.Conn
	r    *bufio.Reader
}

// Follow sends one subscribe request and leaves the connection open.
// Next reads each following line. The caller must Close.
func Follow(ctx context.Context, address, token string, req Request) (*Follower, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	req.Token = token
	conn, err := dial(ctx, address)
	if err != nil {
		if isNotRunning(err) {
			return nil, ErrNotRunning
		}
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if err := writeJSON(conn, req); err != nil {
		_ = conn.Close()
		if isNotRunning(err) {
			return nil, ErrNotRunning
		}
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return &Follower{conn: conn, r: bufio.NewReader(conn)}, nil
}

// SetReadDeadline bounds the next reads. A zero time clears the deadline.
func (f *Follower) SetReadDeadline(deadline time.Time) error {
	if f == nil || f.conn == nil {
		return errors.New("ipc stream is closed")
	}
	return f.conn.SetReadDeadline(deadline)
}

func (f *Follower) Next() (Response, error) {
	if f == nil || f.r == nil {
		return Response{}, errors.New("ipc stream is closed")
	}
	line, err := f.r.ReadBytes('\n')
	if err != nil {
		return Response{}, err
	}
	if len(line) > maxMessage {
		return Response{}, fmt.Errorf("ipc message is too large")
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return Response{}, err
	}
	resp.Running = true
	return resp, nil
}

func (f *Follower) Close() error {
	if f == nil || f.conn == nil {
		return nil
	}
	err := f.conn.Close()
	f.conn = nil
	return err
}

func isNotRunning(err error) bool {
	if err == nil || errors.Is(err, ErrNotRunning) {
		return errors.Is(err, ErrNotRunning)
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ENOENT, syscall.ECONNREFUSED:
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such file") ||
		strings.Contains(msg, "cannot find the file") ||
		strings.Contains(msg, "the system cannot find")
}
