package adapters

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"volvid/internal/core"
)

func TestResetDownloadSlotSendsResetAfterDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ch := make(chan core.DlUpdate, 1)
		done := make(chan struct{})
		go func() {
			resetDownloadSlot(context.Background(), 1, ch)
			close(done)
		}()

		synctest.Wait()
		select {
		case u := <-ch:
			t.Fatalf("unexpected early update %+v", u)
		default:
		}

		time.Sleep(slotResetDelay)
		<-done

		select {
		case u := <-ch:
			if u.Type != core.EvReset || u.Slot != 1 {
				t.Fatalf("unexpected update %+v", u)
			}
		default:
			t.Fatal("expected reset update")
		}
	})
}

func TestResetDownloadSlotCanceledContextSkipsReset(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		ch := make(chan core.DlUpdate, 1)
		done := make(chan struct{})
		go func() {
			resetDownloadSlot(ctx, 2, ch)
			close(done)
		}()

		synctest.Wait()
		<-done

		select {
		case u := <-ch:
			t.Fatalf("unexpected update after cancel: %+v", u)
		default:
		}
	})
}
