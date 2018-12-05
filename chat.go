package main

import (
	"fmt"
	"sync/atomic"
)

func handleNewLeader(id uint64) {
	atomic.StoreUint64(&leaderId, id)

	log(fmt.Sprintf("New leader elected, nodeId=0x%X", id))
	updateStatus()
}

func chatMessage(m string) {

}
