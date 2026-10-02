package ipc

import (
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
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
