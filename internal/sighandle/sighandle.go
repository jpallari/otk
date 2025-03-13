package sighandle

import (
	"context"
	"os"
	"os/signal"
)

func CancelOnSignals(
	ctx context.Context,
	sig ...os.Signal,
) (
	nextCtx context.Context,
	sigCancel func(),
) {
	nextCtx, cancel := context.WithCancel(ctx)
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, sig...)
	sigCancel = func() {
		signal.Stop(sigs)
		cancel()
	}
	go func() {
		select {
		case <-sigs:
			cancel()
		case <-nextCtx.Done():
		}
	}()
	return
}
