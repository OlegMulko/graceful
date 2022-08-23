package graceful

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
)

// SignalHandler...
type SignalHandler func(sig os.Signal, exitCh chan int)

// ErrHandler...
type ErrHandler func(err error, exitCh chan int)

// ShutdownHandler...
type ShutdownHandler func() error

var (
	cancelFn       context.CancelFunc
	signalCh       chan os.Signal              = make(chan os.Signal, 1)
	exitCh         chan int                    = make(chan int)
	signalHandlers map[os.Signal]SignalHandler = map[os.Signal]SignalHandler{
		syscall.SIGINT:  DefaultSignalHandler,
		syscall.SIGTERM: DefaultSignalHandler,
		syscall.SIGQUIT: DefaultSignalHandler,
		syscall.SIGKILL: DefaultSignalHandler,
	}
	DefaultSignalHandler SignalHandler = func(sig os.Signal, exitCh chan int) {
		log.Printf("graceful: get signal %s\n", sig.String())
		exitCh <- 0
	}
	shutdownHandlers []ShutdownHandler = make([]ShutdownHandler, 0)
	errCh            chan error        = make(chan error, 1)
	errHandler       ErrHandler        = func(err error, exitCh chan int) {
		log.Printf("graceful: get error %s\n", err)
		exitCh <- 1
	}
	g errgroup.Group = errgroup.Group{}
)

// Notify...
func Notify(sig ...os.Signal) {

	signal.Notify(signalCh, sig...)

	go func() {

		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("graceful: recovered %v", r)
			}
		}()

	LOOP:
		for {
			select {
			case s := <-signalCh:
				handler, ok := signalHandlers[s]
				if !ok {
					log.Printf("graceful: handler for the signal %s was not found\n", s.String())
					exitCh <- 0
					return
				}
				log.Printf("graceful: get signal %s\n", s.String())
				handler(s, exitCh)
				break LOOP
			case err := <-errCh:
				errHandler(err, exitCh)
			}
		}
	}()
}

// NotifyCtx...
func NotifyCtx(sig ...os.Signal) context.Context {

	var (
		ctx context.Context
	)

	ctx, cancelFn = context.WithCancel(context.Background())

	Notify(sig...)

	return ctx
}

// ErrNotify...
func ErrNotify() chan error {
	return errCh
}

// SetSignalHandler...
func SetSignalHandler(sig os.Signal, handler SignalHandler) {
	signalHandlers[sig] = handler
}

// SetErrHandler...
func SetErrHandler(handler ErrHandler) {
	errHandler = handler
}

// AddShutdownHandler...
func AddShutdownHandler(handler ShutdownHandler) {
	shutdownHandlers = append(shutdownHandlers, handler)
}

func shutdown() {
	for _, handler := range shutdownHandlers {
		g.Go(handler)
	}
}

// Wait ...
func Wait() {

	exitCode := <-exitCh
	defer os.Exit(exitCode)

	log.Printf("graceful: exit code %d\n", exitCode)

	shutdown()

	if cancelFn != nil {
		cancelFn()
	}

	if err := g.Wait(); err == nil {
		log.Printf("graceful: error %v\n", err)
	}
}
