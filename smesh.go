package smesh

import (
	"context"
	"runtime/debug"
	"sync"
	"time"
)

const (
	defaultRecoverDuration = 3 * time.Second
)

// Service is an interface that represents a service that can be ran as
// a separate, independent goroutine.
type Service interface {
	// Run should run the service with the provided context and logger.
	// If context is cancelled, the service should close gracefully.
	// If the service cannot continue running for some reason, it should
	// return an error; upon an error, there can be attempts to retry
	// running the service.
	Run(context.Context)
}

type serviceFunc func(context.Context)

// Run runs the wrapped service function.
func (f serviceFunc) Run(ctx context.Context) {
	f(ctx)
}

// ServiceCloser returns a closing service that should perform the closing
// function when the service is deemed to be closed.
func ServiceCloser(closeFn func()) Service {
	return serviceFunc(func(ctx context.Context) {
		<-ctx.Done()
		closeFn()
	})
}

// Options specifies additional options that can be provided when running
// a service mesh
type Options struct {
	// RecoverDuration specifies the duration after which a service
	// will attempt to recover after panic.
	RecoverDuration time.Duration

	// OnServiceStart is a function that is executed when a service
	// is about to start.
	OnServiceStart func(name string)

	// OnServiceStop is a function that is executed when a service
	// stops. It is not invoked after a panic.
	OnServiceStop func(name string)

	// OnServicePanic is a function that is executed when a service
	// panics.
	OnServicePanic func(name string, stack []byte)
}

// ServiceGroup represents a group of services where the key is service
// name and the value is the service itself.
type ServiceGroup map[string]Service

// Run runs the provided service groups.
// If more than one service group is passed, it will run groups in
// the same order as they were provided.
// Each service group represents a collection of services that are independent.
// Upon context cancellation, a graceful terminate is initiated; it will
// start by closing the last service group and then close the group before
// it. While doing so, it ensures that it begins closing the second group
// if and only if the group before it gracefully closed (i.e., all services
// returned).
// For example, let us say that we have 3 service groups, SG1, SG2, SG3.
// If we do Run(ctx, logger, SG1, SG2, SG3), it will run the groups in
// following sequence: SG1 -> SG2 -> SG3.
// When ctx is cancelled, it will shut down in the following order:
// SG3 -> SG2 -> SG1.
func Run(ctx context.Context, opts Options, groups ...ServiceGroup) {
	if opts.RecoverDuration <= 0 {
		opts.RecoverDuration = defaultRecoverDuration
	}

	var wg sync.WaitGroup

	wg.Add(len(groups))

	prevCancel := func() {}

	for i := range groups {
		gctx, cancel := context.WithCancel(context.Background())
		if i == len(groups)-1 {
			gctx = ctx
		}

		i := i
		go func(ctx context.Context, cancel context.CancelFunc, sg ServiceGroup) {
			sg.run(ctx, opts)

			cancel()
			wg.Done()
		}(gctx, prevCancel, groups[i])

		prevCancel = cancel
	}

	wg.Wait()
}

func (sg ServiceGroup) run(ctx context.Context, opts Options) {
	var wg sync.WaitGroup

	wg.Add(len(sg))

	runService := func(name string, service Service) {
		defer wg.Done()

		retry(ctx, opts.RecoverDuration, func() (retry bool) {
			defer func() {
				if val := recover(); val != nil {
					retry = true
					opts.OnServicePanic(name, debug.Stack())
				}
			}()

			if opts.OnServiceStart != nil {
				opts.OnServiceStart(name)
			}

			service.Run(ctx)

			if opts.OnServiceStop != nil {
				opts.OnServiceStop(name)
			}

			return retry
		})
	}

	for name, service := range sg {
		go runService(name, service)
	}

	wg.Wait()
}

func retry(ctx context.Context, intv time.Duration, fn func() bool) {
	tm := time.NewTimer(0)

	cleanup := func() {
		tm.Stop()

		select {
		case <-tm.C:
		default:
		}
	}

	cleanup()
	defer cleanup()

	for fn() {
		tm.Reset(time.Second * 3)

		select {
		case <-tm.C:
		case <-ctx.Done():
			return
		}
	}
}
