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
	sepchar         = ";"
	magic           = "DISTROCHYA-R1"
	connect         = "connect"     // params=id;requested_relation;params
	netinfo         = "netinfo"     // params=node_id;next_id;leader_id
	closering       = "closering"   // params=sender_id
	election        = "election"    // params=candidate_id
	elected         = "elected"     // params=leader_id
	userlist        = "userlist"    // params=[users]
	chatmessage     = "chatmessage" // params=user;message
	chatmessagesend = "chmsgsend"   // params=message
	nextinfo        = "nextinfo"    // params=next_id
	alivecheck      = "alivecheck"  // no params
	aliveresponse   = "aliveresp"   // no params

	// network states
	noNetwork  = "No Network"
	singleNode = "Single Node"
	ring       = "Ring"

	// other
	ringRepairTimeoutSeconds         = 3
	sendMessageTimeoutSeconds        = 3
	leaderElectionTimeoutSeconds     = 5
	leaderElectionMinimumWaitSeconds = 3
	leaderElectionMaximumWaitSeconds = 15
	connectionTimeoutSeconds         = 20
	connectionTimeoutGraceSeconds    = 5
)

var networkGlobalsMutex = &sync.Mutex{}
var networkStateMutex = &sync.Mutex{}

var server net.Listener
var networkState = noNetwork
var nodeID uint64
var twiceNextNodeID uint64
var nodes *nodeSyncLinkedList

var ringBroken uint32 = 0 // atomic, not guarded by mutex

func updateNetworkState(s string) {
	networkStateMutex.Lock()
	defer networkStateMutex.Unlock()
	log("Network state changed to " + s)
	networkState = s

	if s == singleNode {
		log("NETWORK STATE CHANGED TO SINGLE NODE, ASSUMING LEADER ROLE")
		handleNewLeader(nodeID)
		updateTwiceNextNodeID(0)
	}
}

func getNetworkState() string {
	networkStateMutex.Lock()

	rtn := networkState

	networkStateMutex.Unlock()
	return rtn
}

func updateTwiceNextNodeID(id uint64) {
	atomic.StoreUint64(&twiceNextNodeID, id)
}

func getTwiceNextNodeID() uint64 {
	return atomic.LoadUint64(&twiceNextNodeID)
}

func resetNode() {
	networkGlobalsMutex.Lock()
	defer networkGlobalsMutex.Unlock()

	server = nil
	nodeID = 0
	updateTwiceNextNodeID(0)
	updateLeaderID(0)
	resetChatConnections()
	updateUsers(nil)
	resetConnectedName()

	connectedNodes := nodes.toSlice()

	for _, node := range connectedNodes {
		node.disconnect()
	}

	nodes = nil
	updateNetworkState(noNetwork)
}

func initNode(ip uint32, p uint16) {
	networkGlobalsMutex.Lock()
	defer networkGlobalsMutex.Unlock()

	resetTime()
	updateUsers(nil)

	nodeID = createNodeID(ip, p, uint16(rand.Uint32()))
	updateTwiceNextNodeID(0)
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

func stringToID(s string) (uint64, error) {
	return strconv.ParseUint(s, 16, 64)
}

func isNetworkRunning() bool {
	networkGlobalsMutex.Lock()
	defer networkGlobalsMutex.Unlock()

	return server != nil
}

func broadcastToFollowers(m ...string) {
	networkGlobalsMutex.Lock()
	defer networkGlobalsMutex.Unlock()

	if nodes != nil {
		nodes.lock.Lock()
		defer nodes.lock.Unlock()

		cn := nodes.head

		for cn != nil {
			cn.data.lock.Lock()
			if cn.data.r == follower {
				cn.data.sendMessage(m...)
			}
			cn.data.lock.Unlock()

			cn = cn.next
		}
	}
}

func closeRing(oldNextNodeID uint64) {
	if atomic.LoadUint32(&ringBroken) == 1 {
		return
	}

	if !isNetworkRunning() {
		return
	}

	prevNode := findNodeByRelation(prev)

	if prevNode == nil {
		updateNetworkState(singleNode)
	} else {
		prevNode.lock.Lock()

		if prevNode.id == oldNextNodeID {
			prevNode.lock.Unlock()
			updateNetworkState(singleNode)
		} else {
			atomic.StoreUint32(&ringBroken, 1)
			prevNode.lock.Unlock()

			twiceNextNodeID := getTwiceNextNodeID()
			twiceNextNode := connectToNode(idToEndpoint(twiceNextNodeID))

			if twiceNextNode != nil {
				twiceNextNode.lock.Lock()
				twiceNextNode.id = twiceNextNodeID
			} else {
				log("Connection to twice next node failed")
				prevNode.lock.Lock()
			}

			go func() {
				if twiceNextNode == nil {
					for atomic.LoadUint32(&ringBroken) == 1 && prevNode.connected {
						prevNode.lock.Unlock()
						log("Broken ring detected with failure to connect to twiceNextNode")
						log(fmt.Sprintf("Sending closering: target_id=0x%X, sender_id=0x%X", prevNode.id, nodeID))
						prevNode.sendMessage(closering, idToString(nodeID))
						time.Sleep(ringRepairTimeoutSeconds * time.Second)
						prevNode.lock.Lock()
					}
					prevNode.lock.Unlock()
				} else {
					for atomic.LoadUint32(&ringBroken) == 1 && twiceNextNode.connected {
						twiceNextNode.lock.Unlock()
						log("Broken ring detected with successful connection to twiceNextNode")
						log(fmt.Sprintf("Sending closering: target_id=0x%X, sender_id=0x%X", twiceNextNodeID, nodeID))
						twiceNextNode.sendMessage(closering, idToString(nodeID))
						time.Sleep(ringRepairTimeoutSeconds * time.Second)
						twiceNextNode.lock.Lock()
					}
					twiceNextNode.lock.Unlock()
				}
			}()
		}
	}
}

func findNodeByRelation(r relation) *Node {
	networkGlobalsMutex.Lock()

	var rtn *Node
	if nodes != nil {
		rtn = nodes.findSingleByRelation(r)
	}

	networkGlobalsMutex.Unlock()

	return rtn
}

func findNodeByRelationExcludingID(r relation, id uint64) *Node {
	networkGlobalsMutex.Lock()

	var rtn *Node
	if nodes != nil {
		rtn = nodes.findSingleByRelationExcludingID(r, id)
	}

	networkGlobalsMutex.Unlock()

	return rtn
}

func removeNode(n *Node) {
	networkGlobalsMutex.Lock()
	defer networkGlobalsMutex.Unlock()

	if nodes != nil {
		nodes.remove(n)
	}
}

func addNode(n *Node) {
	networkGlobalsMutex.Lock()
	defer networkGlobalsMutex.Unlock()

	if nodes != nil {
		nodes.add(n)
	}
}

func createNodeID(i uint32, p uint16, f uint16) uint64 {
	return uint64(i)<<32 | uint64(p)<<16 | uint64(f)
}

func disconnect() {
	defer updateStatus()

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

	log(fmt.Sprintf("Server started, listening on port %d. nodeID=0x%X", p, nodeID))
	userEvent(fmt.Sprintf("listening on port %d", p))

	if newNetwork {
		updateNetworkState(singleNode)
	}

	// incoming connections
	for server != nil {
		c, err := server.Accept()

		if err != nil {
			debugLog("Server error: " + err.Error())
			return
		}

		n := nodeFromConnection(c)
		go n.handleConnection()
	}
}

func startNetwork(p uint16) {
	defer updateStatus()

	if isNetworkRunning() {
		userError("already connected")
		return
	}

	serverStartResultChan := make(chan bool)

	initNode(getIp(), p)
	go startServer(p, true, serverStartResultChan)

	if !<-serverStartResultChan {
		resetNode()
	}
}

func joinNetwork(a string, p uint16) {
	if isNetworkRunning() {
		userError("already connected")
		return
	}

	serverStartResultChan := make(chan bool)

	initNode(getIp(), p)
	go startServer(p, false, serverStartResultChan)

	if <-serverStartResultChan {
		node := connectToNode(a)

		if node == nil {
			disconnect()
			userError("Failed to connect to the remote network")
			return
		}
		node.r = prev

		log(fmt.Sprintf("Sending connect message: address=%s, my_id=0x%X, r=%s", a, nodeID, none))
		node.sendMessage(connect, idToString(nodeID), string(none))
	} else {
		resetNode()
	}
}
