package main

import (
	"testing"
)

func TestByteArrayAlloc(t *testing.T ) {
	pool := NewByteArrayPool(100, 64*1024)
	if pool.Size() > 0 {
		t.Fail()
	}
	allocs := make([][]byte, 0 )
	for i := 0; i < 10; i++ {
		b := pool.Alloc()
		allocs = append( allocs, b )
		if len(b) != 64 * 1024 {
			t.Errorf("the allocated array is not 64k")
		}
	}

	for _, b := range( allocs ) {
		pool.Free( b )
	}


	if pool.Size() != 10 {
		t.Errorf("the pool size is not 10")
	}
	c := pool.Alloc()
	if len( c ) != 64 * 1024 {
		t.Errorf("the allocated array size is not 64k")
	}
	pool.Free(c)
	if pool.Size() != 10 {
		t.Errorf("the pool size is not 10")
	}
}
