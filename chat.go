package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var leaderMutex *sync.Mutex = &sync.Mutex{}
var leaderId uint64

func updateLeaderId(id uint64) {
	leaderMutex.Lock()
	defer leaderMutex.Unlock()
	atomic.StoreUint64(&leaderId, id)
}

func readLeaderId() uint64 {
	leaderMutex.Lock()
	defer leaderMutex.Unlock()

	rtn := atomic.LoadUint64(&leaderId)

	return rtn
}

func handleNewLeader(id uint64) {
	updateLeaderId(id)
	log(fmt.Sprintf("New leader elected, nodeId=0x%X", id))
	updateStatus()
}

func chatMessage(m string) {

}
