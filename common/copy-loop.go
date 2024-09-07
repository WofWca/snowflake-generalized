package common

import (
	"io"
	"log"
	"sync"
)

// Copy-pasted from
// https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/-/blob/f4db64612c500be635dc7eb231505e88552e6a07/proxy/lib/snowflake.go#L307-335
// Pipes data between the two connections.
// Returns when piping at either end fails.
func CopyLoop(c1 io.ReadWriteCloser, c2 io.ReadWriteCloser, shutdown chan struct{}) {
	var once sync.Once
	done := make(chan struct{})
	copyer := func(dst io.ReadWriteCloser, src io.ReadWriteCloser) {
		// Experimentally each usage of buffer has been observed to be lower than
		// 2K; io.Copy defaults to 32K.
		size := 2 * 1024
		buffer := make([]byte, size)
		// Ignore io.ErrClosedPipe because it is likely caused by the
		// termination of copyer in the other direction.
		if _, err := io.CopyBuffer(dst, src, buffer); err != nil && err != io.ErrClosedPipe {
			log.Printf("io.CopyBuffer inside CopyLoop generated an error: %v", err)
		}
		once.Do(func() {
			close(done)
		})
	}

	go copyer(c1, c2)
	go copyer(c2, c1)

	select {
	case <-done:
	case <-shutdown:
	}
	log.Println("copy loop ended")
}
