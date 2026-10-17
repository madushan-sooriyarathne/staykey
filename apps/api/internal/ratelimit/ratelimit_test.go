package ratelimit

import (
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestTheRequestPastTheLimitIsRefused(t *testing.T) {
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	l := New(c.now)
	for range 5 {
		l.Allow("ip:1", 5, time.Minute)
	}
	if ok, wait := l.Allow("ip:1", 5, time.Minute); ok || wait != time.Minute {
		t.Errorf("sixth request = %v, wait %v", ok, wait)
	}
}

func TestANewWindowStartsAfresh(t *testing.T) {
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	l := New(c.now)
	for range 5 {
		l.Allow("ip:1", 5, time.Minute)
	}
	c.t = c.t.Add(time.Minute)
	if ok, _ := l.Allow("ip:1", 5, time.Minute); !ok {
		t.Error("request in the next window was refused")
	}
}

func TestKeysCountSeparately(t *testing.T) {
	l := New(nil)
	l.Allow("ip:1", 1, time.Minute)
	if ok, _ := l.Allow("ip:2", 1, time.Minute); !ok {
		t.Error("another key was refused")
	}
}

func TestEndedWindowsAreDropped(t *testing.T) {
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	l := New(c.now)
	l.Allow("ip:1", 5, time.Second)
	c.t = c.t.Add(2 * time.Minute)
	l.Allow("ip:2", 5, time.Second)
	if len(l.windows) != 1 {
		t.Errorf("windows kept = %d, want 1", len(l.windows))
	}
}
