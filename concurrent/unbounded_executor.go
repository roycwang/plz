package concurrent

import (
	"sync"
	"github.com/v2pro/plz/countlog"
	"context"
	"runtime"
	"time"
	"fmt"
)

const StopSignal = "STOP!"

type UnboundedExecutor struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	activeGoroutinesMutex *sync.Mutex
	activeGoroutines      map[string]int
}


// GlobalUnboundedExecutor has the life cycle of the program itself
// any goroutine want to be shutdown before main exit can be started from this executor
var GlobalUnboundedExecutor = NewUnboundedExecutor()

func NewUnboundedExecutor() *UnboundedExecutor {
	ctx, cancel := context.WithCancel(context.TODO())
	return &UnboundedExecutor{
		ctx:                   ctx,
		cancel:                cancel,
		activeGoroutinesMutex: &sync.Mutex{},
		activeGoroutines:      map[string]int{},
	}
}

func (executor *UnboundedExecutor) Go(handler func(ctx context.Context)) {
	_, file, line, _ := runtime.Caller(1)
	executor.activeGoroutinesMutex.Lock()
	defer executor.activeGoroutinesMutex.Unlock()
	startFrom := fmt.Sprintf("%s:%d", file, line)
	executor.activeGoroutines[startFrom] += 1
	go func() {
		defer func() {
			recovered := recover()
			if recovered != nil && recovered != StopSignal {
				countlog.Fatal("event!unbounded_executor.panic",
					"err", recovered,
					"stacktrace", countlog.ProvideStacktrace)
			}
			executor.activeGoroutinesMutex.Lock()
			defer executor.activeGoroutinesMutex.Unlock()
			executor.activeGoroutines[startFrom] -= 1
		}()
		handler(executor.ctx)
	}()
}

func (executor *UnboundedExecutor) Stop() {
	executor.cancel()
}

func (executor *UnboundedExecutor) StopAndWaitForever() {
	executor.StopAndWait(context.Background())
}

func (executor *UnboundedExecutor) StopAndWait(ctx context.Context) {
	executor.cancel()
	for {
		fiveSeconds := time.NewTimer(time.Millisecond * 100)
		select {
		case <-fiveSeconds.C:
		case <-ctx.Done():
			return
		}
		if executor.checkGoroutines() {
			return
		}
	}
}

func (executor *UnboundedExecutor) checkGoroutines() bool {
	executor.activeGoroutinesMutex.Lock()
	defer executor.activeGoroutinesMutex.Unlock()
	for startFrom, count := range executor.activeGoroutines {
		if count > 0 {
			countlog.Info("event!unbounded_executor.still waiting goroutines to quit",
				"startFrom", startFrom,
				"count", count)
			return false
		}
	}
	return true
}