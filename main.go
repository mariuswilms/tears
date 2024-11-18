// Copyright 2024 Marius Wilms All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tears provides a simple way to stack functions to be run on
// teardown. It is useful to ensure that resources are properly released when
// a function returns, even if it returns early due to an error.
package tears

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"
)

// Timeout after which a cleanup function is considered to be stuck.
var Timeout = 15 * time.Second

// TearFn is a function that allows to add a cleanup function.
type TearFn func(c any) Tear

// DownFn is a function that runs the cleanup functions in reverse order.
type DownFn func(context.Context) error

// New returns a pair of tear and down functions. The tear function
// allows to add cleanup functions, the down function runs the cleanup.
func New() (TearFn, DownFn) {
	var cleaner Cleaner
	return cleaner.Tear, cleaner.Down
}

// Cleaner allows to register cleanup functions and run them in reverse order.
// it is not safe for concurrent use. A Cleaner can be embbeded into another
// struct to provide tear-down functionality.
type Cleaner []Tear

type Tear struct {
	fn func(context.Context) error

	// Usually the cleanup functions are run in the reverse order they have been
	// added, and in a FIFO manner. The additonal priority allows to break out of
	// this. By setting a low (maybe even negative) priority the cleanup function
	// will run later. By setting a high priority it will run earlier.
	prio int
}

// End will cause the cleanup function to be run at the end of the cleanup
// phase. When several cleanup function are set to run at the end, they will be
// run in the reverse order they have been added.
func (t *Tear) End() Tear {
	t.prio = -1000
	return *t
}

// Tear accepts a wide range of types that can be used as cleanup functions and
// types. Tear will schedule the cleanup function to be run on Down.
func (c *Cleaner) Tear(v any) Tear {
	var t Tear

	switch v.(type) {
	case func(): //  no context, no error, also covers context.CancelFunc
		t.fn = func(context.Context) error {
			v.(func())()
			return nil
		}
	case func() error: // no context, with error
		t.fn = func(context.Context) error {
			return v.(func() error)()
		}
	case func(context.Context): // with context, no error
		t.fn = func(ctx context.Context) error {
			v.(func(context.Context))(ctx)
			return nil
		}
	case func(context.Context) error: // with context, with error
		t.fn = v.(func(context.Context) error)
	case chan<- bool: // quit-channel
		t.fn = func(context.Context) error {
			v.(chan<- bool) <- true
			return nil
		}
	default:
		panic(fmt.Sprintf("unsupported type %T", v))
	}
	return t
}

// Down runs the cleanup functions in reverse order they have been added.
func (c *Cleaner) Down(ctx context.Context) error {
	errs := make(chan error, len(*c))

	slices.SortFunc(*c, func(i, j Tear) int {
		return cmp.Compare(i.prio, j.prio)
	})
	for i := len(*c) - 1; i >= 0; i-- {
		// Run the cleanup function in a goroutine to prevent a deadlock in case
		// a cleanup function is stalled/blocking.
		done := make(chan bool)

		go func() {
			if err := (*c)[i].fn(ctx); err != nil {
				errs <- err
			}
			done <- true
		}()

		select {
		case <-done:
			break
		case <-time.After(Timeout):
			errs <- fmt.Errorf("timeout")
			break
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d errors encountered, first error: %s", len(errs), <-errs)
	}
	return nil
}
