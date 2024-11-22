// Copyright 2024 Marius Wilms All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.package tears

package tears

import (
	"context"
	"testing"
)

func TestAdddedGetsCalledWithStructEmbed(t *testing.T) {
	m := struct {
		Cleaner
	}{}

	var called bool
	m.Tear(func() error {
		called = true
		return nil
	})
	m.Down(context.Background())
	if !called {
		t.Error("Expected cleanup to be called")
	}
}

func TestAdddedGetsCalledWithVar(t *testing.T) {
	var cl Cleaner

	var called bool
	cl.Tear(func() error {
		called = true
		return nil
	})
	cl.Down(context.Background())
	if !called {
		t.Error("Expected cleanup to be called")
	}
}

func TestDownOrderLIFO(t *testing.T) {
	var cl Cleaner

	var called []int
	cl.Tear(func() error {
		called = append(called, 1)
		return nil
	})
	cl.Tear(func() error {
		called = append(called, 2)
		return nil
	})

	cl.Down(context.Background())

	if called[0] != 2 || called[1] != 1 {
		t.Errorf("Expected cleanup to be called in order 2->1, got %v", called)
	}
}

func TestDownOrderEnd(t *testing.T) {
	var cl Cleaner

	var called []int
	cl.Tear(func() error {
		called = append(called, 1)
		return nil
	})
	cl.Tear(func() error {
		called = append(called, 2)
		return nil
	}).End()

	t.Logf("Tears: %#v", cl)
	cl.Down(context.Background())

	if called[0] != 1 || called[1] != 2 {
		t.Errorf("Expected cleanup to be called in order 1->2, got %v", called)
	}
}
