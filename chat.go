package main

import (
	"fmt"
	"sync/atomic"
)

func HandleNewLeader(id uint64) {
	atomic.SwapUint64(&LeaderId, id)

	Log(fmt.Sprintf("New leader elected, nodeId=0x%X", id))
	UpdateStatus()
}