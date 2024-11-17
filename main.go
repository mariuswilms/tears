// Copyright 2024 Marius Wilms All rights reserved.
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
type Cleaner []cleanupFn

// Tear accepts cleanup functions, quit-chanells, or a context.CancelFunc. It will
// add convert them - if necessary - to cleanup functions and add them to the stack.
func (c *Cleaner) Tear(v any) {
	switch v.(type) {
	case func() error:
		c.AddSyncFunc(v.(func() error))
	case cleanupFn:
		c.AddSyncFunc(v.(cleanupFn))
	case chan<- bool:
		c.AddQuitChan(v.(chan<- bool))
	case context.CancelFunc:
		c.AddCancelFunc(v.(context.CancelFunc))
	default:
		panic(fmt.Sprintf("unsupported type %T", v))
	}
}

// Down runs the cleanup functions in reverse order they have been added.
func (c *Cleaner) Down() error {
	var lerr error

	for i := len(*c) - 1; i >= 0; i-- {
		if (*c)[i] == nil {
			continue
		}
		if err := (*c)[i](); err != nil {
			// Do not stop, continue to try to
			// teardown what is left.
			lerr = err
		}
	}
	if lerr != nil {
		return fmt.Errorf("error/s encountered, last error was: %s", lerr)
	}

	return nil
}

// AddSyncFunc adds a simple CleanupFunc to the stack, which gets called on cleanup.
// Generally AddAsyncFunc should be preferred. Use AddSyncFunc when there is a
// strict order dependency between the cleanup functions.
func (c *Cleaner) AddSyncFunc(fn cleanupFn) {
	*c = append(*c, fn)
}

// AddCancelFunc accepts a context.CancelFunc, once cleanup is requested, the
// cancel func will be called and cancel its context.
func (c *Cleaner) AddCancelFunc(fn context.CancelFunc) {
	*c = append(*c, func() error {
		fn()
		return nil
	})
}

// AddQuitChan accepts a so-called quit-channel, once cleanup is requested
// we will signal the channel to close by sending a boolean value.
func (c *Cleaner) AddQuitChan(ch chan<- bool) {
	*c = append(*c, func() error {
		ch <- true
		return nil
	})
}
