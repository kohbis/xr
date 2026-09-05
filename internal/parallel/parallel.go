// Package parallel runs indexed work concurrently while keeping its output in
// index order.
//
// Commands that operate on every repository want concurrency for speed but must
// not let workers interleave their output. Run gives each item its own buffers
// and flushes them in order, so a concurrent run reads exactly like a
// sequential one.
package parallel

import (
	"bytes"
	"io"
	"sync"
)

// Jobs clamps a requested worker count to the useful range for n items.
func Jobs(jobs, n int) int {
	if jobs < 1 {
		jobs = 1
	}
	if jobs > n {
		jobs = n
	}
	return jobs
}

// Results calls fn once for each of the n items, using at most jobs concurrent
// workers, and returns what fn produced for each item in index order.
//
// It is the counterpart of Run for work that returns a value instead of writing
// output: the caller reports the results itself, so ordering is restored by the
// index rather than by flushing buffers. fn runs on several goroutines at once
// and must not touch shared state.
func Results[T any](n, jobs int, fn func(i int) T) []T {
	results := make([]T, n)
	if Jobs(jobs, n) < 2 {
		for i := range n {
			results[i] = fn(i)
		}
		return results
	}

	queue := make(chan int)
	go func() {
		for i := range n {
			queue <- i
		}
		close(queue)
	}()

	var wg sync.WaitGroup
	for range Jobs(jobs, n) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each worker writes only to its own index, so the slice needs no
			// further synchronisation.
			for i := range queue {
				results[i] = fn(i)
			}
		}()
	}
	wg.Wait()

	return results
}

// Run calls fn once for each of the n items, using at most jobs concurrent
// workers, and writes everything fn produces to stdout and stderr in index
// order.
//
// With fewer than two effective workers, fn receives stdout and stderr directly
// so its output streams live. Above that, fn receives buffers that are flushed
// once every earlier item has been flushed: the output is identical, but each
// item's block appears only after that item finishes.
//
// The two streams are buffered separately so redirection keeps working;
// ordering between them is not preserved within a single item.
func Run(n, jobs int, stdout, stderr io.Writer, fn func(i int, out, err io.Writer)) {
	if Jobs(jobs, n) < 2 {
		for i := range n {
			fn(i, stdout, stderr)
		}
		return
	}

	type slot struct {
		out  bytes.Buffer
		err  bytes.Buffer
		done chan struct{}
	}

	slots := make([]*slot, n)
	for i := range slots {
		slots[i] = &slot{done: make(chan struct{})}
	}

	queue := make(chan int)
	go func() {
		for i := range n {
			queue <- i
		}
		close(queue)
	}()

	var wg sync.WaitGroup
	for range Jobs(jobs, n) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				s := slots[i]
				fn(i, &s.out, &s.err)
				// Publishes the buffers to the flushing loop.
				close(s.done)
			}
		}()
	}

	for _, s := range slots {
		<-s.done
		_, _ = io.Copy(stdout, &s.out)
		_, _ = io.Copy(stderr, &s.err)
	}
	wg.Wait()
}
