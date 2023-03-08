package flower

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Run(t *testing.T) {
	testCases := map[string]struct {
		Groups []map[string]*serviceStub
	}{
		"Closes in order": {
			Groups: []map[string]*serviceStub{
				{
					"S1": &serviceStub{},
					"S2": &serviceStub{},
					"S3": &serviceStub{},
					"S4": &serviceStub{},
				},
				{
					"S5": &serviceStub{},
					"S6": &serviceStub{},
					"S7": &serviceStub{},
					"S8": &serviceStub{},
				},
				{
					"S9":  &serviceStub{},
					"S10": &serviceStub{},
					"S11": &serviceStub{},
					"S12": &serviceStub{},
				},
			},
		},
		"Closes in order and recovers from panic": {
			Groups: []map[string]*serviceStub{
				{
					"S1": &serviceStub{},
					"S2": &serviceStub{},
					"S3": &serviceStub{
						panic: true,
					},
					"S4": &serviceStub{},
				},
				{
					"S5": &serviceStub{},
					"S6": &serviceStub{
						panic: true,
					},
					"S7": &serviceStub{},
					"S8": &serviceStub{},
				},
				{
					"S9":  &serviceStub{},
					"S10": &serviceStub{},
					"S11": &serviceStub{},
					"S12": &serviceStub{
						panic: true,
					},
				},
			},
		},
	}

	for caseName, testCase := range testCases {
		testCase := testCase

		t.Run(caseName, func(t *testing.T) {
			t.Parallel()

			var (
				resultMu sync.Mutex

				serviceCount int
				result       []serviceExit
				converted    []ServiceGroup
			)

			startedCh := make(chan struct{})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			for i, group := range testCase.Groups {
				sg := make(ServiceGroup)
				converted = append(converted, sg)

				for name, service := range group {
					name := name
					i := i

					service.closedFn = func() {
						resultMu.Lock()
						result = append([]serviceExit{{Service: name, Group: i}}, result...)
						resultMu.Unlock()
					}

					service.startedFn = func() {
						startedCh <- struct{}{}
					}

					sg[name] = service
					serviceCount++
				}
			}

			doneCh := make(chan struct{})

			go func() {
				defer close(doneCh)
				Run(ctx, Options{
					RecoverDuration: time.Millisecond,
				}, converted...)
			}()

			for i := 0; i < serviceCount; i++ {
				<-startedCh
			}

			cancel()
			<-doneCh

			require.Len(t, result, serviceCount, "not all services closed: %s", formatServiceExits(result...))
			for i, resIndex := 0, 0; i < len(testCase.Groups); i++ {
				for range testCase.Groups[i] {
					assert.Equal(
						t,
						i,
						result[resIndex].Group,
						"service %s should have been closed in group %d, but closed in %d (sequence: %s)",
						result[resIndex].Service,
						result[resIndex].Group,
						i,
						formatServiceExits(result...),
					)

					resIndex++
				}
			}
		})
	}
}

type serviceExit struct {
	Group   int
	Service string
}

func formatServiceExits(se ...serviceExit) string {
	var queue strings.Builder
	queue.WriteString("[ ")
	for i := range se {
		el := se[len(se)-1-i]

		queue.WriteString(fmt.Sprintf("%d:%s ", el.Group, el.Service))
	}
	queue.WriteString("]")

	return queue.String()
}

type serviceStub struct {
	mu     sync.Mutex
	panic  bool
	closed bool

	closedFn  func()
	startedFn func()
}

func (ss *serviceStub) Run(ctx context.Context) {
	if ss.panic {
		ss.panic = false
		panic("forced panic")
	}

	ss.startedFn()

	<-ctx.Done()
	ss.mu.Lock()
	ss.closed = true
	ss.mu.Unlock()

	ss.closedFn()
}
