package netwatch

import (
	"context"
	"testing"
	"time"
)

func resetPublisher() {
	pubMu.Lock()
	pub = publisher{}
	pubMu.Unlock()
}

func TestPublishNoListenerDoesNotBlock(t *testing.T) {
	resetPublisher()
	t.Cleanup(resetPublisher)

	done := make(chan struct{})
	go func() {
		publish(KindResume)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked with no listener")
	}
}

func TestPublishAfterClearDoesNotSendOnClosedChannel(t *testing.T) {
	ctx := context.Background()
	ch := make(chan Event)
	setPublisher(ctx, ch)
	t.Cleanup(resetPublisher)

	clearPublisher(ctx)
	close(ch)
	publish(KindNetwork)
	publish(KindResume)
}

func TestPublishDropsWhenNobodyReceives(t *testing.T) {
	ctx := context.Background()
	ch := make(chan Event)
	setPublisher(ctx, ch)
	t.Cleanup(func() { clearPublisher(ctx) })

	done := make(chan struct{})
	go func() {
		publish(KindResume)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked the caller")
	}
	select {
	case ev := <-ch:
		t.Fatalf("delivered without a waiting receiver: %+v", ev)
	default:
	}
}

func TestPublishDropsWhenBufferFull(t *testing.T) {
	ctx := context.Background()
	ch := make(chan Event, 1)
	setPublisher(ctx, ch)
	t.Cleanup(func() { clearPublisher(ctx) })

	publish(KindResume)
	done := make(chan struct{})
	go func() {
		publish(KindNetwork)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked on a full buffer")
	}
	if got := len(ch); got != 1 {
		t.Fatalf("buffer len = %d, want 1", got)
	}
}

func TestPublishStopsAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan Event, 1)
	setPublisher(ctx, ch)
	t.Cleanup(func() { clearPublisher(ctx) })

	publish(KindResume)
	select {
	case ev := <-ch:
		if ev.Kind != KindResume {
			t.Fatalf("kind = %q", ev.Kind)
		}
	default:
		t.Fatal("expected a resume event")
	}

	cancel()
	publish(KindNetwork)
	select {
	case ev := <-ch:
		t.Fatalf("sent after stop: %+v", ev)
	default:
	}
}

func TestClearPublisherLeavesNewerListener(t *testing.T) {
	older, cancelOlder := context.WithCancel(context.Background())
	newer, cancelNewer := context.WithCancel(context.Background())
	defer cancelOlder()
	defer cancelNewer()
	ch := make(chan Event, 1)
	setPublisher(older, make(chan Event, 1))
	setPublisher(newer, ch)
	t.Cleanup(resetPublisher)

	clearPublisher(older)
	publish(KindNetwork)
	select {
	case ev := <-ch:
		if ev.Kind != KindNetwork {
			t.Fatalf("kind = %q", ev.Kind)
		}
	default:
		t.Fatal("newer listener was cleared")
	}
}
