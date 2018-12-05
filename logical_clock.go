package main

import (
	"sync"
	"sync/atomic"
)

var currentTime uint64 = 0
var timeLock *sync.Mutex = &sync.Mutex{}

func advanceTime() uint64 {
	timeLock.Lock()
	defer timeLock.Unlock()

	rtn := atomic.AddUint64(&currentTime, 1)

	return rtn
}

func updateTime(t uint64) uint64 {
	timeLock.Lock()
	defer timeLock.Unlock()

	time := atomic.LoadUint64(&currentTime)

	if t > time {
		time = t
	}

	time++

	atomic.StoreUint64(&currentTime, time)

	return time
}

func resetTime() {
	timeLock.Lock()
	defer timeLock.Unlock()

	atomic.StoreUint64(&currentTime, 0)
}

func readTime() uint64 {
	timeLock.Lock()
	defer timeLock.Unlock()

	rtn := atomic.LoadUint64(&currentTime)
	return rtn
}
