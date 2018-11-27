package main

import "fmt"

func HandleNewLeader(id uint64) {
	Log(fmt.Sprintf("New leader elected, nodeId=0x%X", id))
	UpdateStatus()
}