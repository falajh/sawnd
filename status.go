package main

import (
	"sync"
	"time"
)

// PlaybackState is the high-level state of mpv's playback core.
type PlaybackState int

const (
	StatePlaying PlaybackState = iota
	StatePaused
	StateStopped
)

// PlayerStatus is a snapshot of mpv's playback state, bubbled up from the
// audio backend to anything that cares (MPRIS, lyrics sync, etc).
type PlayerStatus struct {
	State     PlaybackState
	Position  time.Duration
	Length    time.Duration
	VolumePct int
}

// StatusBroadcaster fans a single stream of PlayerStatus updates out to any
// number of independent subscribers. This is what lets the audio backend
// stay ignorant of MPRIS, lyrics sync, or anything else that reacts to
// playback state — it just publishes, and doesn't know or care who's
// listening.
type StatusBroadcaster struct {
	mu   sync.Mutex
	subs []chan PlayerStatus
}

func NewStatusBroadcaster() *StatusBroadcaster {
	return &StatusBroadcaster{}
}

// Subscribe returns a channel that receives every future status update.
// The channel is buffered so a slow/absent reader can never block
// publishing or other subscribers; if a subscriber falls behind, the
// unread update is replaced with the newest one rather than queued.
func (b *StatusBroadcaster) Subscribe() <-chan PlayerStatus {
	ch := make(chan PlayerStatus, 1)
	b.mu.Lock()
	b.subs = append(b.subs, ch)
	b.mu.Unlock()
	return ch
}

// latest-value / overwrite buffer
func (b *StatusBroadcaster) publish(s PlayerStatus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- s:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- s:
			default:
			}
		}
	}
}

func (b *StatusBroadcaster) closeAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		close(ch)
	}
	b.subs = nil
}
