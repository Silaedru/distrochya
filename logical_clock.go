package main

import (
	"sync"
)

var currentTime uint64 = 0
var timeLock = &sync.Mutex{}

func advanceTime() uint64 {
	timeLock.Lock()

	currentTime++
	rtn := currentTime

	timeLock.Unlock()

	return rtn
}

func updateTime(t uint64) uint64 {
	timeLock.Lock()

	time := currentTime

	if t > time {
		time = t
	}

	time++

	currentTime = time

	timeLock.Unlock()

	return time
}

func resetTime() {
	timeLock.Lock()

	currentTime = 0

	timeLock.Unlock()
}

func getTime() uint64 {
	timeLock.Lock()

	rtn := currentTime

	timeLock.Unlock()

	return rtn
}
