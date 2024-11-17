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

// CleanupFunc is a function that closes resources and
// performs a general cleanup.
type CleanupFunc func() error

// Teardown stacks functions for teardown.
type Teardown struct {
	Scope string

	// Debug, if set than this function will receive debug messages. This can
	// be used to log debug messages to a logger.
	//  &Teardown{ Debugger: log.Print }
	Debug func(msg string)

	// fns is a stack of CleanupFuncs to run on Close().
	fns []CleanupFunc

	// wg is a WaitGroup to block until all async CleanupFuncs have
	// finished executing.
	wg sync.WaitGroup
}

func (td *Teardown) String() string {
	return fmt.Sprintf("teardown (%s)", td.Scope)
}

func (td *Teardown) debugf(format string, v ...interface{}) {
	if td.Debug != nil {
		td.Debug(fmt.Sprintf(format, v...))
	}
}

// AddFunc adds a simple CleanupFunc to the stack, which gets called on cleanup.
// Generally AddAsyncFunc should be preferred. Use AddFunc when there is a
// strict order dependency between the cleanup functions.
func (td *Teardown) AddFunc(fn CleanupFunc) {
	td.fns = append(td.fns, func() error {
		td.debugf("Running %s func %s...", td, funcName(fn))
		start := time.Now()

		err := fn()

		td.debugf("Ran %s func %s in %s", td, funcName(fn), time.Since(start))
		return err
	})
}

// AddAsyncFunc adds a CleanupFunc to the stack, which gets
// called asynchronously on cleanup. This approach should be used
// whenever there is no order dependency between the cleanup functions.
func (td *Teardown) AddAsyncFunc(fn CleanupFunc) {
	td.fns = append(td.fns, func() error {
		td.wg.Add(1)

		go func() {
			td.debugf("Running %s async-func %s...", td, funcName(fn))
			start := time.Now()

			fn()
			td.wg.Done()

			td.debugf("Ran %s async-func %s in %s", td, funcName(fn), time.Since(start))
		}()

		return nil
	})
}

// AddCancelFunc accepts a context.CancelFunc, once cleanup is requested, the
// cancel func will be called and cancel its context.
func (td *Teardown) AddCancelFunc(fn context.CancelFunc) {
	td.fns = append(td.fns, func() error {
		td.debugf("Running %s cancel-func %s...", td, funcName(fn))
		start := time.Now()

		fn()

		td.debugf("Ran %s cancel-func %s in %s", td, funcName(fn), time.Since(start))
		return nil
	})
}

// AddChan accepts a so-called quit-channel, once cleanup is requested
// we will signal the channel to close by sending a boolean value.
func (td *Teardown) AddChan(ch chan<- bool) {
	td.fns = append(td.fns, func() error {
		td.debugf("Running %s chan-close-func...", td)
		start := time.Now()

		ch <- true

		td.debugf("Ran %s chan-close-func in %s", td, time.Since(start))
		return nil
	})
}

// Close runs the teardown funcs in reverse order they have been added.
func (td *Teardown) Close() error {
	var lerr error

	for i := len(td.fns) - 1; i >= 0; i-- {
		if td.fns[i] == nil {
			continue
		}
		if err := td.fns[i](); err != nil {
			// Do not stop, continue to try to
			// teardown what is left.
			td.debugf("Failed to %s: %s", td, err)
			lerr = err
		}
	}
	if lerr != nil {
		return fmt.Errorf("error/s encountered in %s, last error was: %s", td, lerr)
	}

	td.wg.Wait()
	td.debugf("Successfully completed %s with %d func/s", td, len(td.fns))
	return nil
}

func funcName(fn interface{}) string {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	return strings.Replace(name, "github.com/mariuswilms/teardown/", "", 1)
}
