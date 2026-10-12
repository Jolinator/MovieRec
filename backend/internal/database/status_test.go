package database

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakePinger struct {
	err   atomic.Value // error
	calls atomic.Int32
}

func (p *fakePinger) set(err error) { p.err.Store(&err) }

func (p *fakePinger) Ping(context.Context) error {
	p.calls.Add(1)
	if v := p.err.Load(); v != nil {
		return *v.(*error)
	}
	return nil
}

func TestDBStatusScan(t *testing.T) {
	pinger := &fakePinger{}
	status := NewDBStatus(pinger)
	if status.Available() {
		t.Fatal("new status should be unavailable")
	}

	status.Scan(context.Background())
	if !status.Available() {
		t.Fatal("expected available after successful ping")
	}

	pinger.set(errors.New("down"))
	status.Scan(context.Background())
	if status.Available() {
		t.Fatal("expected unavailable after failed ping")
	}
}

func TestDBStatusScanKeepsStateWhenContextCancelled(t *testing.T) {
	pinger := &fakePinger{}
	status := NewDBStatus(pinger)
	status.Scan(context.Background())

	pinger.set(errors.New("down"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	status.Scan(ctx)
	if !status.Available() {
		t.Fatal("cancelled scan must keep the last known state")
	}
}

func TestDBStatusRefreshOnlyWhenUnavailable(t *testing.T) {
	pinger := &fakePinger{}
	status := NewDBStatus(pinger)

	status.Refresh(context.Background())
	if pinger.calls.Load() != 1 || !status.Available() {
		t.Fatalf("calls=%d available=%v", pinger.calls.Load(), status.Available())
	}
	status.Refresh(context.Background())
	if pinger.calls.Load() != 1 {
		t.Fatalf("refresh pinged an available database (%d calls)", pinger.calls.Load())
	}
}

func TestDBStatusStartScansPeriodicallyUntilCancelled(t *testing.T) {
	pinger := &fakePinger{}
	status := NewDBStatus(pinger)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		status.Start(ctx, 10*time.Millisecond)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for pinger.calls.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if pinger.calls.Load() < 3 {
		t.Fatalf("only %d scans", pinger.calls.Load())
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start did not stop")
	}
}

func TestDBStatusStartDefaultsInterval(t *testing.T) {
	pinger := &fakePinger{}
	status := NewDBStatus(pinger)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		status.Start(ctx, 0)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done
	if pinger.calls.Load() != 1 {
		t.Fatalf("calls = %d, want the single initial scan", pinger.calls.Load())
	}
}
