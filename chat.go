package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"math/rand"
)

var leaderId uint64
var oldLeaderId uint64
var chatParticipation uint32 = 1
var chatNameMutex *sync.Mutex = &sync.Mutex{}
var chatName string = "test"

var leaderElectionMutex *sync.Mutex = &sync.Mutex{}
var leaderElectionTimer *time.Timer
var electionParticipated uint32
var electionStartTriggerFlag uint32

func startElectionTimer(timeout uint8) {
	if readNetworkState() == singleNode {
		log("Attempt to start election timer with networkState==singleNode, assuming leader role")
		updateLeaderId(nodeId)
		return
	}

	leaderElectionMutex.Lock()
	defer leaderElectionMutex.Unlock()

	if leaderElectionTimer != nil {
		return
	}

	log(fmt.Sprintf("startElectionTimer timeout=%ds", timeout))
	leaderElectionTimer := time.NewTimer(time.Duration(timeout) * time.Second)

	go func() {
		<- leaderElectionTimer.C
		
		if readLeaderId() == 0 {
			log("Absence of leader detected")
			setElectionParticipated()

			nextNode := findNodeByRelation(next)
			if nextNode == nil {
				log("Absence of leader detected without having next node")
			} else {
				log(fmt.Sprintf("Absence of leader detected: sending election, target_id=0x%X, candidate_id=0x%X", nextNode.id, nodeId))
				nextNode.sendMessage(election, idToString(nodeId))
			}
			resetElectionTimer()
		}
	}()
}

func updateLeaderId(id uint64) {
	atomic.StoreUint64(&oldLeaderId, readLeaderId())
	atomic.StoreUint64(&leaderId, id)

	if server == nil {
		return
	}

	if id == 0 {
		resetElectionParticipated()
		resetElectionStartTriggerFlag()
		stopElectionTimer()

		startElectionTimer(uint8(leaderElectionMinimumWait + (rand.Uint32() % (leaderElectionMaximumWait-leaderElectionMinimumWait))))
	} else {
		resetElectionParticipated()
		resetElectionStartTriggerFlag()
		stopElectionTimer()
	}
}

func hasElectionParticipated() bool {
	return atomic.LoadUint32(&electionParticipated) != 0
}

func resetElectionParticipated() {
	atomic.StoreUint32(&electionParticipated, 0)
}

func setElectionParticipated() {
	atomic.StoreUint32(&electionParticipated, 1)
}

func readLeaderId() uint64 {
	return atomic.LoadUint64(&leaderId)
}

func getOldLeaderId() uint64 {
	return atomic.LoadUint64(&oldLeaderId)
}

func isElectionStartTriggerFlagSet() bool {
	return atomic.LoadUint32(&electionStartTriggerFlag) != 0
}

func setElectionStartTriggerFlag() {
	atomic.StoreUint32(&electionStartTriggerFlag, 1)
}

func resetElectionStartTriggerFlag() {
	atomic.StoreUint32(&electionStartTriggerFlag, 0)
}

func setChatParticipation() {
	atomic.StoreUint32(&chatParticipation, 1)
}

func getChatParticipation() uint32 {
	return atomic.LoadUint32(&chatParticipation)
}

func resetChatParticipation() {
	atomic.StoreUint32(&chatParticipation, 0)
}

func stopElectionTimer() {
	leaderElectionMutex.Lock()
	defer leaderElectionMutex.Unlock()

	if leaderElectionTimer != nil {
		leaderElectionTimer.Stop()
	}

	leaderElectionTimer = nil
}

func resetElectionTimer() {
	stopElectionTimer()
	startElectionTimer(leaderElectionTimeoutSeconds)
}

func setChatName(n string) {
	chatNameMutex.Lock()
	chatName = n
	chatNameMutex.Unlock()
}

func getChatName() string {
	chatNameMutex.Lock()
	rtn := chatName
	chatNameMutex.Unlock()

	return rtn
}

func connectToLeader() {
	/*if !isNetworkRunning() {
		return
	}*/

	if len(getChatName()) < 1 {
		userError("Unable to connect: no chat nickname set")
		return
	}

	existingLeader := findNodeByRelation(leader)

	if existingLeader != nil {
		existingLeader.disconnect()
	}

	newLeaderId := readLeaderId()
	newLeader := connectToClient(idToEndpoint(newLeaderId))

	if newLeader == nil {
		updateLeaderId(0)
		log("Connection to a new leader failed")
		return
	}

	newLeader.lock.Lock()
	newLeader.id = newLeaderId
	newLeader.r = leader
	newLeader.lock.Unlock()
	newLeader.sendMessage(connect, idToString(nodeId), string(follower), getChatName())
}

func handleNewLeader(id uint64) {
	defer updateStatus()

	updateLeaderId(id)

	log(fmt.Sprintf("New leader elected, nodeId=0x%X", id))

	if getChatParticipation() == 1 {
		connectToLeader()
	}
}

func chatMessage(m string) {

}
