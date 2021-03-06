package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var leaderID uint64
var oldLeaderID uint64
var chatParticipation uint32 = 1
var chatNameMutex = &sync.Mutex{}
var chatName = "User"

var leaderElectionMutex = &sync.Mutex{}
var leaderElectionTimer *time.Timer
var electionParticipated uint32
var electionStartTriggerFlag uint32

func startElectionTimer(t uint8) {
	if getNetworkState() == singleNode {
		log("Attempt to start election timer with networkState==singleNode, assuming leader role")
		updateLeaderID(nodeID)
		return
	}

	leaderElectionMutex.Lock()
	defer leaderElectionMutex.Unlock()

	if leaderElectionTimer != nil {
		return
	}

	log(fmt.Sprintf("startElectionTimer timeout=%ds", t))
	leaderElectionTimer := time.NewTimer(time.Duration(t) * time.Second)

	go func() {
		<-leaderElectionTimer.C

		if getLeaderID() == 0 {
			log("Absence of leader detected")
			setElectionParticipated()

			nextNode := findNodeByRelation(next)
			if nextNode == nil {
				log("Absence of leader detected without having next node")

				prevNode := findNodeByRelation(prev)

				if prevNode == nil {
					log("Absence of leader detected without having next or prev node, falling back to singleNode")
					updateNetworkState(singleNode)
				} else {
					log("Absence of leader detected without having next node: Awaiting ring repair")
				}
			} else {
				log(fmt.Sprintf("Absence of leader detected: sending election, target_id=0x%X, candidate_id=0x%X", nextNode.id, nodeID))
				nextNode.sendMessage(election, idToString(nodeID))
			}
			resetElectionTimer()
		}
	}()
}

func updateLeaderID(id uint64) {
	atomic.StoreUint64(&oldLeaderID, getLeaderID())
	atomic.StoreUint64(&leaderID, id)

	if server == nil {
		return
	}

	if id == 0 {
		resetElectionParticipated()
		resetElectionStartTriggerFlag()
		stopElectionTimer()

		startElectionTimer(uint8(leaderElectionMinimumWaitSeconds + (rand.Uint32() % (leaderElectionMaximumWaitSeconds - leaderElectionMinimumWaitSeconds))))
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

func getLeaderID() uint64 {
	return atomic.LoadUint64(&leaderID)
}

func getOldLeaderID() uint64 {
	return atomic.LoadUint64(&oldLeaderID)
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

	if isNetworkRunning() {
		connectToLeader()
		appendChatView(fmt.Sprintf("\x1b[33mYou have joined the chat\x1b[0m"))
	}
}

func getChatParticipation() uint32 {
	return atomic.LoadUint32(&chatParticipation)
}

func resetChatParticipation() {
	atomic.StoreUint32(&chatParticipation, 0)

	if isNetworkRunning() {
		disconnectFromLeader()
		updateUsers(nil)
		appendChatView(fmt.Sprintf("\x1b[33mYou have left the chat\x1b[0m"))
	}
}

func stopElectionTimer() {
	leaderElectionMutex.Lock()
	defer leaderElectionMutex.Unlock()

	if leaderElectionTimer != nil {
		leaderElectionTimer.Stop()
		log("Election timer stopped")
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
	if len(getChatName()) < 1 {
		userError("Unable to connect: no chat nickname set")
		return
	}

	disconnectFromLeader()

	newLeaderID := getLeaderID()
	newLeader := connectToNode(idToEndpoint(newLeaderID))

	if newLeader == nil {
		updateLeaderID(0)
		log("Connection to a new leader failed")
		return
	}

	newLeader.lock.Lock()
	newLeader.id = newLeaderID
	newLeader.r = leader
	newLeader.lock.Unlock()
	newLeader.sendMessage(connect, idToString(nodeID), string(follower), getChatName())

	setConnectedName(getChatName())
}

func disconnectFromLeader() {
	resetConnectedName()

	existingLeader := findNodeByRelation(leader)

	if existingLeader != nil {
		existingLeader.lock.Lock()
		existingLeader.r = none
		existingLeader.lock.Unlock()

		existingLeader.disconnect()
	}
}

func handleNewLeader(id uint64) {
	defer updateStatus()

	resetChatConnections()
	updateLeaderID(id)

	log(fmt.Sprintf("New leader elected, nodeID=0x%X", id))

	if getChatParticipation() == 1 {
		connectToLeader()
	}
}

func chatMessage(m string) {
	if isNetworkRunning() {
		if getChatParticipation() > 0 {
			leader := findNodeByRelation(leader)

			if leader != nil {
				log(fmt.Sprintf("Sending chatmessagesend, target_id=0x%X", leader.id))
				leader.sendMessage(chatmessagesend, m)
			} else {
				userError("cannot send your message because there is no leader on the network, please wait a few moments and then try again")
			}
		} else {
			userError("you are not participating in the chat")
		}
	} else {
		userError("you are not connected to any network")
	}
}
