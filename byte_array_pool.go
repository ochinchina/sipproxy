package main

import (
	"sync"
)

type ByteArrayPool struct {
	sync.Mutex
	maxCap int
	arraySize int
	pool [][]byte
}

func NewByteArrayPool( maxCap, arraySize int )*ByteArrayPool {
	return &ByteArrayPool{ maxCap: maxCap, arraySize: arraySize, pool: make([][]byte, 0 )}
}

func (bp *ByteArrayPool)Alloc() []byte {
	bp.Lock()
	defer bp.Unlock()

	n := len( bp.pool )
	if n <= 0 {
		return make([]byte, bp.arraySize )
	}
	r := bp.pool[n-1]
	bp.pool = bp.pool[0:n-1]
	return r
}

func (bp *ByteArrayPool)Size() int {
	bp.Lock()
        defer bp.Unlock()

	return len( bp.pool )
}

func (bp *ByteArrayPool)Free(b []byte) {
	bp.Lock()
        defer bp.Unlock()
	n := len( bp.pool )
	if n <= 0 || n < bp.maxCap {
		bp.pool = append( bp.pool, b )
	}
}
