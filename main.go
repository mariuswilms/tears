// Copyright 2024 Marius Wilms All rights reserved.
// Copyright 2020 Marius Wilms, Christoph Labacher. All rights reserved.
// Copyright 2019 Atelier Disko. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tears provides a simple way to stack functions to be run on
// teardown. It is useful to ensure that resources are properly released when
// a function returns, even if it returns early due to an error.
package tears

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

// cleanupFn is a function that closes resources and
// performs a general cleanup.
type cleanupFn func() error

// tearFn is a function that allows to add a cleanup function.
type tearFn func(c any)

// downFn is a function that runs the cleanup functions in reverse order.
type downFn func() error

// New returns a pair of tear and down functions. The tear function
// allows to add cleanup functions, the down function runs the cleanup.
func New() (tearFn, downFn) {
	var cleaner Cleaner
	return cleaner.Tear, cleaner.Down
}

// Cleaner allows to register cleanup functions and run them in reverse order.
// it is not safe for concurrent use. A Cleaner can be embbeded into another
// struct to provide tear-down functionality.
//
//	type MyApp struct {
//		tears.Cleaner
//	}
//
//	func (a *MyApp) Run() {
//		a.Tear(/* ...*/)
//	}
//
//	func a (a *MyApp) Close() {
//		a.Down()
//	}
type Cleaner struct {
	// fns is a stack of CleanupFuncs to run on Close().
	fns []cleanupFn

	// wg is a WaitGroup to block until all async CleanupFuncs have
	// finished executing. It is used internally on final cleanup.
	wg sync.WaitGroup

	// Debug, if set than this function will receive debug messages. This can
	// be used to log debug messages to a logger, i.e.
	// tears.Cleaner{ Debug: log.Print }
	Debug func(v ...any)
}

func (td *Cleaner) debugf(format string, v ...interface{}) {
	if td.Debug != nil {
		td.Debug(fmt.Sprintf(format, v...))
	}
}

func (td *Cleaner) Tear(c any) {
	switch c.(type) {
	case func() error:
		td.AddSyncFunc(c.(func() error))
	case cleanupFn:
		td.AddSyncFunc(c.(cleanupFn))
	case chan<- bool:
		td.AddChan(c.(chan<- bool))
	case context.CancelFunc:
		td.AddCancelFunc(c.(context.CancelFunc))
	default:
		panic(fmt.Sprintf("unsupported type %T", c))
	}
}

// AddAsyncFunc adds a CleanupFunc to the stack, which gets
// called asynchronously on cleanup. This approach should be used
// whenever there is no order dependency between the cleanup functions.
func (td *Cleaner) AddAsyncFunc(fn cleanupFn) {
	td.fns = append(td.fns, func() error {
		td.wg.Add(1)

		go func() {
			td.debugf("Running async-func %s...", funcName(fn))
			start := time.Now()

			fn()
			td.wg.Done()

			td.debugf("Ran async-func %s in %s", funcName(fn), time.Since(start))
		}()

		return nil
	})
}

// AddSyncFunc adds a simple CleanupFunc to the stack, which gets called on cleanup.
// Generally AddAsyncFunc should be preferred. Use AddSyncFunc when there is a
// strict order dependency between the cleanup functions.
func (td *Cleaner) AddSyncFunc(fn cleanupFn) {
	td.fns = append(td.fns, func() error {
		td.debugf("Running func %s...", funcName(fn))
		start := time.Now()

		err := fn()

		td.debugf("Ran func %s in %s", funcName(fn), time.Since(start))
		return err
	})
}

// AddCancelFunc accepts a context.CancelFunc, once cleanup is requested, the
// cancel func will be called and cancel its context.
func (td *Cleaner) AddCancelFunc(fn context.CancelFunc) {
	td.fns = append(td.fns, func() error {
		td.debugf("Running cancel-func %s...", funcName(fn))
		start := time.Now()

		fn()

		td.debugf("Ran cancel-func %s in %s", funcName(fn), time.Since(start))
		return nil
	})
}

// AddChan accepts a so-called quit-channel, once cleanup is requested
// we will signal the channel to close by sending a boolean value.
func (td *Cleaner) AddChan(ch chan<- bool) {
	td.fns = append(td.fns, func() error {
		td.debugf("Running chan-close-func...")
		start := time.Now()

		ch <- true

		td.debugf("Ran chan-close-func in %s", time.Since(start))
		return nil
	})
}

// Down runs the teardown funcs in reverse order they have been added.
func (td *Cleaner) Down() error {
	var lerr error

	for i := len(td.fns) - 1; i >= 0; i-- {
		if td.fns[i] == nil {
			continue
		}
		if err := td.fns[i](); err != nil {
			// Do not stop, continue to try to
			// teardown what is left.
			td.debugf("Failed: %s", err)
			lerr = err
		}
	}
	if lerr != nil {
		return fmt.Errorf("error/s encountered, last error was: %s", lerr)
	}

	td.wg.Wait()
	td.debugf("Successfully completed with %d func/s", len(td.fns))
	return nil
}

func funcName(fn interface{}) string {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	return strings.Replace(name, "github.com/mariuswilms/teardown/", "", 1)
}
