package worker

import (
	"runtime"

	"golang.org/x/net/context"
)

type Pool struct {
	ctxCancelFunc context.CancelFunc
	queue         chan func()
}

type Pools map[string]*Pool

const (
	Browser  = "browser"
	Renderer = "renderer"
)

func New(ctx context.Context, maxWorker int) *Pool {
	if maxWorker <= 0 {
		maxWorker = runtime.NumCPU()
	}

	queue := make(chan func(), maxWorker)
	ctx, cancel := context.WithCancel(ctx)

	for range maxWorker {
		go func() {
			for {
				select {
				case f := <-queue:
					f()
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	return &Pool{cancel, queue}
}

func (w *Pool) Do(f func()) {
	w.queue <- f
}

func (w *Pool) Done() {
	w.ctxCancelFunc()
}
