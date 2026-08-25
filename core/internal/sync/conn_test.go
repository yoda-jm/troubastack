package sync

import (
	stdsync "sync"
	"testing"
)

// T106 — the send channel has two independent closers: broadcast dropping a slow consumer, and
// readPump's teardown. closeSend must make that close happen EXACTLY once no matter how they race.
// Before the atomic guard this was safe only emergently (an ordering across three call sites); revert
// closeSend to a plain close(c.send) and this test panics with "close of closed channel" under -race.
func TestConnCloseSendExactlyOnce(t *testing.T) {
	for iter := 0; iter < 500; iter++ {
		c := &conn{send: make(chan []byte, 1)}
		var wg stdsync.WaitGroup
		const closers = 8
		wg.Add(closers)
		for i := 0; i < closers; i++ {
			go func() {
				defer wg.Done()
				c.closeSend()
			}()
		}
		wg.Wait()

		// Exactly one close happened: the channel reads as closed (never blocks, ok=false) and no
		// "close of closed channel" panic took the test down.
		if _, ok := <-c.send; ok {
			t.Fatal("send channel should be closed after closeSend")
		}
	}
}
