package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type relation string

const (
	// node relations
	none     = relation("none")
	next     = relation("next")
	prev     = relation("prev")
	leader   = relation("leader")
	follower = relation("follower")

	// messages
	// magic;message;params\n
	magic     = "d"
	connect   = "con"       // params=id;requested_relation
	netinfo   = "nei"       // params=node_id;next_id;leader_id
	closering = "closering" // params=sender_id

	// network states
	noNetwork  = "noNetwork"
	singleNode = "singleNode"
	ring       = "ring"

	// other
	ringRepairTimeoutSeconds  = 3
	sendMessageTimeoutSeconds = 3
)

var networkGlobalsMutex *sync.Mutex = &sync.Mutex{}
var networkStateMutex *sync.Mutex = &sync.Mutex{}

var server net.Listener
var networkState string
var nodeId uint64
var nodes *nodeSyncLinkedList

var ringBroken uint32 = 0 // atomic, not guarded by mutex

func updateNetworkState(s string) {
	networkStateMutex.Lock()
	defer networkStateMutex.Unlock()
	log("Network state changed to " + s)
	networkState = s
}

func readNetworkState() string {
	networkStateMutex.Lock()

	rtn := networkState

	networkStateMutex.Unlock()
	return rtn
}

func resetNode() {
	networkGlobalsMutex.Lock()
	defer networkGlobalsMutex.Unlock()

	server = nil
	nodeId = 0
	updateNetworkState(noNetwork)
	updateLeaderId(0)

	connectedNodes := nodes.toSlice()
	
	for _, node := range(connectedNodes) {
		node.disconnect()
	}

	nodes = nil
}

func initNode(ip uint32, p uint16) {
	networkGlobalsMutex.Lock()
	defer networkGlobalsMutex.Unlock()

	resetTime()
	rand.Seed(time.Now().UnixNano())
	nodeId = createNodeId(ip, p, uint16(rand.Uint32()))
	nodes = newNodeSyncLinkedList()
}

func getIp() uint32 {
	nicAddrs, err := net.InterfaceAddrs()

	if err == nil {
		for _, nicAddr := range nicAddrs {
			ip, ok := nicAddr.(*net.IPNet)
			ipv4 := ip.IP.To4()

			if ok && !ip.IP.IsLoopback() && ipv4 != nil {
				var rtn uint32
				binary.Read(bytes.NewBuffer(ipv4), binary.BigEndian, &rtn)
				return rtn
			}
		}
	}

	return 0xFFFFFFFF
}

func idToEndpoint(id uint64) string {
	port := (id & 0x00000000FFFF0000) >> 16
	ip := (id & 0xFFFFFFFF00000000) >> 32

	return fmt.Sprintf("%d.%d.%d.%d:%d", (ip>>24)&0xFF, (ip>>16)&0xFF, (ip>>8)&0xFF, ip&0xFF, port)
}

func idToString(id uint64) string {
	return strconv.FormatUint(id, 16)
}

func stringToId(s string) (uint64, error) {
	return strconv.ParseUint(s, 16, 64)
}

func closeRing(oldNextNodeId uint64) {
	if atomic.LoadUint32(&ringBroken) == 1 {
		return
	}

	networkGlobalsMutex.Lock()
	var prevNode *Node
	if nodes != nil {
		prevNode = nodes.findSingleByRelation(prev)
	}
	networkGlobalsMutex.Unlock()

	if prevNode == nil {
		updateNetworkState(singleNode)
	} else {
		prevNode.lock.Lock()

		if prevNode.id == oldNextNodeId {
			prevNode.lock.Unlock()
			updateNetworkState(singleNode)
		} else {
			atomic.StoreUint32(&ringBroken, 1)

			go func() {
				for atomic.LoadUint32(&ringBroken) == 1 && prevNode.connected {
					prevNode.lock.Unlock()
					log("Broken ring detected")
					log(fmt.Sprintf("Sending closering: target_id=0x%X, sender_id=0x%X", prevNode.id, nodeId))
					prevNode.sendMessage(closering, idToString(nodeId))
					time.Sleep(ringRepairTimeoutSeconds * time.Second)
					prevNode.lock.Lock()
				}

				prevNode.lock.Unlock()
			}()
		}
	}
}

func createNodeId(i uint32, p uint16, f uint16) uint64 {
	return uint64(i)<<32 | uint64(p)<<16 | uint64(f)
}

func disconnect() {
	networkGlobalsMutex.Lock()
	if server == nil {
		userError("not connected to any network")
		networkGlobalsMutex.Unlock()
		return
	}
	networkGlobalsMutex.Unlock()

	server.Close()
	resetNode()

	userEvent("disconnected")
	log("Server stopped")
	updateStatus()
}

func startServer(p uint16, newNetwork bool, resultChan chan bool) {
	l, err := net.Listen("tcp4", fmt.Sprintf(":%d", p))

	if err != nil {
		userError(err.Error())
		resultChan <- false
		return
	}

	resultChan <- true

	networkGlobalsMutex.Lock()
	server = l
	networkGlobalsMutex.Unlock()

	log(fmt.Sprintf("Server started, listening on port %d. nodeId=0x%X", p, nodeId))
	userEvent(fmt.Sprintf("listening on port %d", p))

	if newNetwork {
		updateNetworkState(singleNode)
		log("NEW NETWORK -> ASSUMING LEADER ROLE")
		handleNewLeader(nodeId)
	}

	// incoming connections
	for server != nil {
		c, err := server.Accept()

		if err != nil {
			debugLog("Server error: " + err.Error())
			return
		}

		n := nodeFromConnection(c)
		go n.handleClient()
	}
}

func startNetwork(p uint16) {
	if server != nil {
		userError("already connected")
		return
	}

	serverStartResultChan := make(chan bool)

	initNode(getIp(), p)
	go startServer(p, true, serverStartResultChan)

	if !<-serverStartResultChan {
		resetNode()
	}

	updateStatus()
}

func joinNetwork(a string, p uint16) {
	if server != nil {
		userError("already connected")
		return
	}

	serverStartResultChan := make(chan bool)

	initNode(getIp(), p)
	go startServer(p, false, serverStartResultChan)

	if <-serverStartResultChan {
		networkNode := connectToClient(a)

		if networkNode == nil {
			disconnect()
			userError("Failed to connect to the remote network")
			return
		}
		networkNode.r = prev

		log(fmt.Sprintf("Sending connect message: address=%s, my_id=0x%X, r=%s", a, nodeId, none))
		networkNode.sendMessage(connect, idToString(nodeId), string(none))
	} else {
		resetNode()
	}
}
