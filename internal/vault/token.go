package vault

import (
	"context"
	"errors"

	vaultapi "github.com/hashicorp/vault/api"
)

// TokenWatcher wraps the Vault SDK's LifetimeWatcher to deliver renewal
// events through a small, channel-based surface that's friendly to a
// Bubble Tea Cmd loop. The watcher renews the current client token at
// the appropriate cadence; consumers receive a single event per renewal
// (or terminal error) on Events().
type TokenWatcher struct {
	w *vaultapi.LifetimeWatcher
}

// WatchEvent is the per-renewal payload delivered to consumers.
type WatchEvent struct {
	// Renewed is non-nil on a successful renewal.
	Renewed *vaultapi.RenewOutput
	// Err is non-nil if the watcher terminated with an error or the
	// token can no longer be renewed.
	Err error
	// Done is true when the watcher has stopped permanently (either
	// because the token can no longer be renewed or the caller stopped
	// it). After a Done event the channel will be closed.
	Done bool
}

// NewTokenWatcher returns a watcher for the *current* client token. The
// client must already be configured with a renewable token. Increment is
// the requested renew increment in seconds; pass 0 to use Vault's
// default.
func (a *apiClient) NewTokenWatcher(increment int) (*TokenWatcher, error) {
	tok := a.c.Token()
	if tok == "" {
		return nil, errors.New("vault: cannot watch unauthenticated client")
	}
	secret := &vaultapi.Secret{
		Auth: &vaultapi.SecretAuth{
			ClientToken:   tok,
			Renewable:     true,
			LeaseDuration: increment,
		},
	}
	w, err := a.c.NewLifetimeWatcher(&vaultapi.LifetimeWatcherInput{
		Secret:    secret,
		Increment: increment,
	})
	if err != nil {
		return nil, err
	}
	return &TokenWatcher{w: w}, nil
}

// Run starts the watcher and forwards events on the returned channel.
// Cancel ctx to stop the watcher; the channel will receive a final Done
// event and be closed.
func (w *TokenWatcher) Run(ctx context.Context) <-chan WatchEvent {
	ch := make(chan WatchEvent, 4)
	go w.w.Start()
	go func() {
		defer close(ch)
		defer w.w.Stop()
		for {
			select {
			case <-ctx.Done():
				ch <- WatchEvent{Done: true}
				return
			case err := <-w.w.DoneCh():
				ch <- WatchEvent{Err: err, Done: true}
				return
			case ro := <-w.w.RenewCh():
				if ro == nil {
					continue
				}
				// Copy to avoid retaining SDK internals.
				cp := *ro
				ch <- WatchEvent{Renewed: &cp}
			}
		}
	}()
	return ch
}

// Stop terminates the watcher.
func (w *TokenWatcher) Stop() { w.w.Stop() }
