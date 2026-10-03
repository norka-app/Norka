package forward

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/norka-app/Norka/internal/model"
)

func TestProbeChainDoesNotSendPassword(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	const secret = "super-secret-value"
	_, err := ProbeChain(ctx, []model.Jumper{{
		Host:     "192.0.2.1",
		Port:     22,
		User:     "root",
		AuthType: "password",
		Password: secret,
	}}, "10.0.0.1", 5432)
	if !errors.Is(err, ErrPasswordNotProbed) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("password leaked into %v", err)
	}
}

func TestDialTimeoutFromJumper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		timeoutMs int
		want      time.Duration
	}{
		{name: "default", timeoutMs: 0, want: 5 * time.Second},
		{name: "negative", timeoutMs: -1, want: 5 * time.Second},
		{name: "custom", timeoutMs: 8000, want: 8 * time.Second},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := dialTimeoutFromJumper(model.Jumper{TimeoutMs: tt.timeoutMs})
			if got != tt.want {
				t.Fatalf("dialTimeoutFromJumper() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDialTimeoutFromJumpers(t *testing.T) {
	t.Parallel()

	if got := dialTimeoutFromJumpers(nil); got != 5*time.Second {
		t.Fatalf("empty jumpers = %v, want 5s", got)
	}

	jumpers := []model.Jumper{
		{TimeoutMs: 3000},
		{TimeoutMs: 7000},
	}
	if got := dialTimeoutFromJumpers(jumpers); got != 7*time.Second {
		t.Fatalf("last jumper timeout = %v, want 7s", got)
	}
}
