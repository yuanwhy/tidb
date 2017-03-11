package bytespool

import (
	"bytes"
	"sync"
)

// BytesPool maintains large bytes pools, used for reducing memory allocation.
// It has a slice of pools which handle different size of bytes.
// Can be safely used concurrently.
type BytesPool struct {
	buckets []sync.Pool
}

const (
	kilo       = 1024
	mega       = kilo * kilo
	baseSize   = kilo
	numBuckets = 18
	maxSize    = 128 * mega
)

// DefaultPool is a default BytesBool instance.
var DefaultPool = NewBytesPool()

// NewBytesPool creates a new bytes pool.
func NewBytesPool() *BytesPool {
	bp := new(BytesPool)
	bp.buckets = make([]sync.Pool, numBuckets)
	for i := uint(0); i < numBuckets; i++ {
		bp.buckets[i].New = makeNewFunc(i)
	}
	return bp
}

func makeNewFunc(shift uint) func() interface{} {
	return func() interface{} {
		return make([]byte, baseSize<<shift)
	}
}

// Alloc allocates a bytes which has the size of power of two.
// The caller should keep the origin bytes and use the returned data.
// When finished using, the origin bytes should be freed to the pool.
// The allocated data may not have zero value.
func (bp *BytesPool) Alloc(size int) (origin, data []byte) {
	if size > maxSize {
		return nil, make([]byte, size)
	}
	var i uint
	for size > (baseSize << i) {
		i++
	}
	origin = bp.buckets[i].Get().([]byte)
	data = origin[:size]
	return
}

// Free frees the data which should be the original bytes return by Alloc.
// It returns the bucket index of the data. returns -1 means the data is not returned to the pool.
func (bp *BytesPool) Free(origin []byte) int {
	originLen := len(origin)
	if originLen > maxSize || originLen < baseSize || !isPowerOfTwo(originLen) {
		return -1
	}
	var i uint
	for originLen > (baseSize << i) {
		i++
	}
	bp.buckets[i].Put(origin)
	return int(i)
}

func isPowerOfTwo(x int) bool {
	return x&(x-1) == 0
}

// ReadCloser frees the origin bytes when Close is called.
type ReadCloser struct {
	pool   *BytesPool
	origin []byte
	buffer *bytes.Buffer
	closed bool
}

// Close implements the io.ReadCloser interface.
// It frees the origin bytes allocated from the pool
func (r *ReadCloser) Close() error {
	if r.closed {
		return nil
	}
	r.pool.Free(r.origin)
	r.closed = true
	return nil
}

// Read implements the io.ReadCloser interface.
func (r *ReadCloser) Read(b []byte) (n int, err error) {
	return r.buffer.Read(b)
}

// SharedBytes returns shared bytes of the underlying buffer.
// The returned bytes may not be used after Close is called.
// And it may not be used at the same time with Read.
func (r *ReadCloser) SharedBytes() []byte {
	return r.buffer.Bytes()
}

// NewReadCloser creates a ReadCloser.
// origin should be allocated from the pool, data can be any slice of the origin.
func NewReadCloser(pool *BytesPool, origin, data []byte) *ReadCloser {
	r := &ReadCloser{
		pool:   pool,
		origin: origin,
		buffer: bytes.NewBuffer(data),
	}
	return r
}