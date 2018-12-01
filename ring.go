package main

import (
	"net"
	"fmt"
	"bufio"
	"bytes"
	"strings"
	"strconv"
	"time"
	"sync/atomic"
	"encoding/binary"
)

type Node struct {
	Id uint64
	relation string
	connection net.Conn
	connected bool
}

const (
	// node relations
	none = "none"
	next = "next"
	prev = "prev"
	leader = "leader"
	follower = "follower"

	// messages
	// magic;message;params\n
	magic = "d"
	connect = "con"  				// params=id;requested_relation
	netinfo = "nei"					// params=node_id;next_id;leader_id
	closering = "closering"			// params=sender_id

	// network states
	noNetwork = "noNetwork"
	singleNode = "singleNode"
	ring = "ring"

	// other
	ringRepairTimeoutSeconds = 3
)

var server net.Listener
var NetworkState string
var NodeId uint64
var LeaderId uint64
var Nodes *NodeSyncLinkedList

var ringBroken uint32 = 0

func resetNode() {
	server = nil
	NetworkState = noNetwork
	NodeId = 0
	LeaderId = 0

	for Nodes.head != nil {
		cn := Nodes.head		
		cn.data.disconnect()
	}

	Nodes = nil
}

func initNode(ip uint32, p uint16) {
	resetTime()
	CreateNodeId(ip, p, 0)
	Nodes = NewNodeSyncLinkedList()
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

func IdToEndpoint(id uint64) string {
	port := (id & 0x00000000FFFF0000) >> 16;
	ip := (id & 0xFFFFFFFF00000000) >> 32;

	return fmt.Sprintf("%d.%d.%d.%d:%d", (ip >> 24) & 0xFF, (ip >> 16) & 0xFF, (ip >> 8) & 0xFF, ip & 0xFF, port)
}

func IdToString(id uint64) string {
	return strconv.FormatUint(id, 16)
}

func StringToId(s string) (uint64, error) {
	return strconv.ParseUint(s, 16, 64)
}

func (n *Node) handleClient() {
	n.connected = true
	Nodes.Add(n)
	Log("New connection!")

	r := bufio.NewReader(n.connection)

	for {
		UpdateStatus()
		data, err := r.ReadString('\n')

		if err != nil {
			Log(fmt.Sprintf("Client 0x%X disconnected", n.Id))
			DebugLog(fmt.Sprintf("0x%X: %s", n.Id, err.Error()))

			n.handleDisconnect()
			return
		}
		data = strings.TrimSpace(data)

		if !n.processMessage(data) {
			n.disconnect()
			Log(fmt.Sprintf("Invalid message from client 0x%X, disconnecting", n.Id))
			Log("IMSG: " + data)
		}

		DebugLog("RECV: ==" + data + "==")
	}
}

func (n *Node) disconnect() {
	n.connection.Close()
}

func (n *Node) sendMessage(m ...string) {
	msg := fmt.Sprintf("%s;%d", magic, advanceTime())

	for _, s := range(m) {
		msg += ";" + s
	}

	DebugLog("SEND: ==" + msg + "==")
	n.connection.Write([]byte(msg + "\n"))
}

// returns false on failure
func (n *Node) processMessage(m string) bool {
	msg := strings.Split(m, ";")

	if len(msg) < 3 || msg[0] != magic {
		return false
	} else {
		recvdTime, err := strconv.ParseUint(msg[1], 10, 64)

		if err != nil {
			DebugLog("processMessage timestamp parse failure")
			return false
		}

		time := updateTime(recvdTime)
		parseStartIx := 3

		switch msg[2] {

			// node would like to connect
			case connect:
				id, err := StringToId(msg[parseStartIx])
				if err != nil {
					DebugLog("CONNECT id err")
					return false
				}

				n.Id = id
				n.relation = msg[parseStartIx+1]

				Log(fmt.Sprintf("[%d] Received connect message: remote_id=0x%X, relation=%s", time, n.Id, n.relation))
				n.handleConnect()

			// response to initial connect request containing basic network info
			case netinfo:
				// will happen when response to closering connection is received
				nodeId, err := StringToId(msg[parseStartIx])
				if err != nil {
					DebugLog("NETINFO nodeId err")
					return false
				}

				nextId, err := StringToId(msg[parseStartIx+1])
				if err != nil {
					DebugLog("NETINFO nextId err")
					return false
				}

				leaderId, err := StringToId(msg[parseStartIx+2])
				if err != nil {
					DebugLog("NETINFO leaderId err")
					return false
				}

				n.Id = nodeId
				Log(fmt.Sprintf("[%d] Received netinfo: remote_id=0x%X, next_id=0x%X, leader_id=0x%X", time, nodeId, nextId, leaderId))
				
				Log(fmt.Sprintf("Attempting to connect to remote node, id=0x%X", nodeId))
				nextNode := Connect(IdToEndpoint(nextId))
				if nextNode == nil {
					Log(fmt.Sprintf("Connection to remote node failed!, id=0x%X", nodeId))
					return false
				}
				nextNode.relation = next
				nextNode.Id = nextId
				Log(fmt.Sprintf("Connection to remote node was successful, id=0x%X", nextId))

				// notify the next node who we are
				Log(fmt.Sprintf("Sending connect message: target_id=0x%X, my_id=0x%X, relation=%s", nextNode.Id, NodeId, prev))
				nextNode.sendMessage(connect, strconv.FormatUint(NodeId, 16), prev)
				
				// if we have connected to somebody we have a ring
				NetworkState = ring
				Log("Network state changed to ring")

				// in case there was a leader in the network
				if leaderId != 0 {
					HandleNewLeader(leaderId)
				}

			case closering:
				senderId, err := StringToId(msg[parseStartIx])
				if err != nil {
					DebugLog("CLOSERING sender id failure")
					return false
				}

				Log(fmt.Sprintf("Received closering: from_id=0x%X, sender_id=0x%X", n.Id, senderId))
				prevNode := Nodes.FindSingleByRelation(prev)

				if prevNode != nil {
					if senderId != NodeId {
						Log(fmt.Sprintf("Forwarding closering: target_id=0x%X, sender_id=0x%X", prevNode.Id, senderId))
						prevNode.sendMessage(closering, msg[parseStartIx])
					} else {
						atomic.StoreUint32(&ringBroken, 0)
						Log(fmt.Sprintf("Closering propagation stopped: target_id=0x%X == sender_id=0x%X", prevNode.Id, senderId))
					}
				} else {
					Log(fmt.Sprintf("Received closering without having a prevNode! from_id=0x%X, sender_id=0x%X", n.Id, senderId))
					Log(fmt.Sprintf("Sending connect message: target_id=0x%X, my_id=0x%X, relation=%s", senderId, NodeId, next))
					prevNode = Connect(IdToEndpoint(senderId))

					if prevNode == nil {
						Log(fmt.Sprintf("Connection to remote node failed!, id=0x%X", senderId))
						return false
					}

					prevNode.relation = prev
					prevNode.Id = senderId
					prevNode.sendMessage(connect, IdToString(NodeId), next)
				}
		}
	}

	return true
}

func (n *Node) handleDisconnect() {
	n.connected = false
	Nodes.Remove(n.Id)

	// connection to next node lost
	if n.relation == next {
		prevNode := Nodes.FindSingleByRelation(prev)

		// if there is no prev node or it was the same one as next
		if prevNode == nil || prevNode.Id == n.Id {
			// then we're left alone
			NetworkState = singleNode
			Log(fmt.Sprintf("Network state changed to singleNode"))
		} else {
			atomic.StoreUint32(&ringBroken, 1)

			go func() {
				for atomic.LoadUint32(&ringBroken) == 1 && prevNode.connected {
					Log("Broken ring detected")
					Log(fmt.Sprintf("Sending closering: target_id=0x%X, sender_id=0x%X", prevNode.Id, NodeId))
					prevNode.sendMessage(closering, IdToString(NodeId))
					time.Sleep(ringRepairTimeoutSeconds * time.Second)
				}
			}()
		}
	}

	UpdateStatus()
}

func (n *Node) handleConnect() {
	if n.relation == none {
		n.relation = next

		// new node is joining the network
		if NetworkState == singleNode {
			n.sendMessage(netinfo, IdToString(NodeId), IdToString(NodeId), IdToString(LeaderId))
			NetworkState = ring
			Log("Network state changed to ring")
		} else {
			oldNext := Nodes.FindSingleByRelation(next)
			if oldNext == nil {
				panic("invalid network state: ring without next node")
			}
			Log(fmt.Sprintf("New next connection while in ring, closing oldNext; old_next_id=0x%X, new_next_id=0x%X", oldNext.Id, n.Id))
			oldNext.relation = none
			oldNext.disconnect()

			n.sendMessage(netinfo, IdToString(NodeId), IdToString(oldNext.Id), IdToString(LeaderId))
		}
	} else if n.relation == prev{
	} else if n.relation == next {
		atomic.StoreUint32(&ringBroken, 0)
	} else {
		panic("invalid connection request")
	}
}

func CreateNodeId(i uint32, p uint16, f uint16) {
	NodeId = uint64(i) << 32 | uint64(p) << 16 | uint64(f)
}

func Connect(a string) *Node {
	c, err := net.Dial("tcp4", a)

	if err != nil {
		UserError(err.Error())
		return nil
	}

	n := &Node{0, none, c, false}
	go n.handleClient()

	return n
}

func Disconnect() {
	if server == nil {
		UserError("not connected to any network")
		return
	}
	
	server.Close()
	resetNode()

	UserEvent("disconnected")
	Log("Server stopped")
	UpdateStatus()
}

func startServer(p uint16, newNetwork bool, resultChan chan bool) {
	l, err := net.Listen("tcp4", fmt.Sprintf(":%d", p))

	if err != nil {
		UserError(err.Error())
		resultChan <- false
		return
	}

	resultChan <- true
	server = l
	Log(fmt.Sprintf("Server started, listening on port %d. nodeId=0x%X", p, NodeId))
	
	if newNetwork {
		NetworkState = singleNode
		UserEvent(fmt.Sprintf("network created, listening on port %d", p))
		Log("NEW NETWORK -> ASSUMING LEADER ROLE")
		HandleNewLeader(NodeId)
	}

	UpdateStatus()

	// incoming connections
	for server != nil {
		c, err := server.Accept()

		if err != nil {
			DebugLog("Server error: " + err.Error())
			return
		}

		n := &Node{0, none, c, false}
		go n.handleClient()
	}
}

func StartNetwork(p uint16) {
	if server != nil {
		UserError("already connected")
		return
	}

	serverStartResultChan := make(chan bool)

	initNode(getIp(), p)
	go startServer(p, true, serverStartResultChan)
	
	if !<- serverStartResultChan {
		resetNode()
	}
}

func JoinNetwork(a string, p uint16) {
	if server != nil {
		UserError("already connected")
		return
	}

	serverStartResultChan := make(chan bool)

	initNode(getIp(), p)
	go startServer(p, false, serverStartResultChan)

	if <- serverStartResultChan {
		networkNode := Connect(a)

		if networkNode == nil {
			Disconnect()
			UserError("Failed to connect to the remote network")
			return
		}
		networkNode.relation = prev

		Log(fmt.Sprintf("Sending connect message: address=%s, my_id=0x%X, relation=%s", a, NodeId, none))
		networkNode.sendMessage(connect, IdToString(NodeId), none)
	} else {
		resetNode()
	}
}