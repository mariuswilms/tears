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
