package main

import (
	"fmt"
	"sync/atomic"
)

var leaderId uint64

func updateLeaderId(id uint64) {
	atomic.StoreUint64(&leaderId, id)
}

func readLeaderId() uint64 {
	return atomic.LoadUint64(&leaderId)
}

func handleNewLeader(id uint64) {
	updateLeaderId(id)
	log(fmt.Sprintf("New leader elected, nodeId=0x%X", id))
	updateStatus()
}

func chatMessage(m string) {

}
