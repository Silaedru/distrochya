package main

import (
	"net"
	"fmt"
	"bufio"
	"bytes"
	"strings"
	"strconv"
	"encoding/binary"
)

type Node struct {
	Id uint64
	relation uint8
	state uint8
	connection net.Conn
}

// node relations
const (
	none = iota

	next
	twiceNext

	prev
	twicePrev

	leader
	follower
)

// node states
const (
	new = iota
	assimilated
)

// network states
const (
	noNetwork = iota
	singleNode
	twoNodes
	ring
)

// messages
// magic;message;params\n
const (
	magic = "d"

	connect = "con"  				// params=id;requested_relation
	netinfo = "nei"					// params=my_id;next_id;twicenext_id;leader_id
	requestNextNode = "rnn"			// params=
	nextNodeInfo = "nni"			// params=next_node_id
)

var server net.Listener
var NetworkState uint8
var NodeId uint64
var LeaderId uint64
var Nodes *NodeSyncLinkedList

func resetNode() {
	server = nil
	NetworkState = 0
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

func (n *Node) handleClient() {
	Nodes.Add(n)
	Log("New client connected")

	r := bufio.NewReader(n.connection)

	for {
		UpdateStatus()
		data, err := r.ReadString('\n')

		if err != nil {
			Log(fmt.Sprintf("Client 0x%X: connection lost", n.Id))
			DebugLog(fmt.Sprintf("0x%X: %s", n.Id, err.Error()))

			n.handleDisconnect()
			return
		}
		data = strings.TrimSpace(data)

		if !n.processMessage(data) {
			n.disconnect()
			Log(fmt.Sprintf("Invalid message from client 0x%X, disconnecting", n.Id))
			DebugLog("IMSG: " + data)
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
				id, err := strconv.ParseUint(msg[parseStartIx], 10, 64)
				if err != nil {
					return false
				}

				n.Id = id

				rel, err := strconv.ParseUint(msg[parseStartIx+1], 10, 8)
				if err != nil {
					return false
				}

				n.relation = uint8(rel)

				Log(fmt.Sprintf("%d: Received connect message, remote_id=%X, relation=%d", time, id, rel))
				n.handleConnect()

			// response to initial connect request containing basic network info
			case netinfo:
				nodeId, err := strconv.ParseUint(msg[parseStartIx], 10, 64)
				if err != nil {
					DebugLog("NETINFO nodeId err")
					return false
				}

				nextId, err := strconv.ParseUint(msg[parseStartIx+1], 10, 64)
				if err != nil {
					DebugLog("NETINFO nextId err")
					return false
				}

				twiceNextId, err := strconv.ParseUint(msg[parseStartIx+2], 10, 64)
				if err != nil {
					DebugLog("NETINFO nextId err")
					return false
				}

				leaderId, err := strconv.ParseUint(msg[parseStartIx+3], 10, 64)
				if err != nil {
					DebugLog("NETINFO leaderId err")
					return false
				}

				n.Id = nodeId
				Log(fmt.Sprintf("%d: Received netinfo message, remote_id=%X, next_id=%X, twice_next_id=%X, leader_id=%X", time, nodeId, nextId, twiceNextId, leaderId))
					
				nextNode := Connect(IdToEndpoint(nextId))
				if nextNode == nil {
					DebugLog("NETINFO nextNode connect failed")
					return false
				}
				nextNode.state = assimilated
				nextNode.relation = next
				nextNode.Id = nextId
				Log(fmt.Sprintf("Connection to remote node was successful, remote_id=%X", nextId))

				// notify the next node who we are
				nextNode.sendMessage(connect, strconv.FormatUint(NodeId, 10), strconv.Itoa(prev))
				Log(fmt.Sprintf("Connection to twice remote node was successful, remote_id=%X", nextId))

				// if the next node is this node then there are only 2 nodes in total
				if nextNode.Id == n.Id {
					NetworkState = twoNodes
				} else {
					NetworkState = ring

					if twiceNextId == 0 {
						twiceNextId = Nodes.FindSingleByRelation(prev).Id
					}


					twiceNextNode := Connect(IdToEndpoint(twiceNextId))
					if twiceNextNode == nil {
						DebugLog("NETINFO twiceNextNode connect failed")
						return false
					}
					twiceNextNode.state = assimilated
					twiceNextNode.relation = twiceNext
					twiceNextNode.Id = twiceNextId

					// notify the twice next node who we are
					twiceNextNode.sendMessage(connect, strconv.FormatUint(NodeId, 10), strconv.Itoa(twicePrev))
					
				}

				if leaderId != 0 {
					HandleNewLeader(leaderId)
				}

			case requestNextNode:
				nextNode := Nodes.FindSingleByRelation(next)

				if nextNode != nil {
					//panic("invalid state")
					n.sendMessage(nextNodeInfo, strconv.FormatUint(nextNode.Id, 10))
				}

			case nextNodeInfo:
				nodeId, err := strconv.ParseUint(msg[parseStartIx], 10, 64)
				if err != nil {
					DebugLog("nextNodeInfo nodeId err")
					return false
				}

				//if NetworkState == twoNodes {
				/*twiceNextNode := Nodes.FindSingleByRelation(twiceNext)					
				if twiceNextNode != nil {
					twiceNextNode.disconnect()
				}*/
				if NetworkState == twoNodes {
					twiceNextNode := Connect(IdToEndpoint(nodeId))
					twiceNextNode.state = assimilated
					twiceNextNode.relation = twiceNext
					twiceNextNode.Id = nodeId

					twiceNextNode.sendMessage(connect, strconv.FormatUint(NodeId, 10), strconv.Itoa(twicePrev))
					NetworkState = ring
				}
				//}
		}
	}

	return true
}

func (n *Node) handleDisconnect() {
	Nodes.Remove(n.Id)
	UpdateStatus()
}

func (n *Node) handleConnect() {
	if n.relation == next {
		// new node is joining the network
		if NetworkState == singleNode {
			n.sendMessage(netinfo, strconv.FormatUint(NodeId, 10), strconv.FormatUint(NodeId, 10), strconv.Itoa(0), strconv.FormatUint(LeaderId, 10))
			n.state = assimilated
			NetworkState = twoNodes
		} else {
			var oldTwiceNextId uint64

			if NetworkState == ring {
				oldTwiceNext := Nodes.FindSingleByRelation(twiceNext)
				if oldTwiceNext == nil {
					panic("invalid network state: ring without twiceNext node")
				}				
				oldTwiceNextId = oldTwiceNext.Id
				oldTwiceNext.disconnect()
			}

			oldNext := Nodes.FindSingleByRelation(next)
			if oldNext == nil {
				panic("invalid network state: twoNodes or ring without next node")
			}

			oldNext.relation = twiceNext
			oldNext.sendMessage(connect, strconv.FormatUint(NodeId, 10), strconv.Itoa(twicePrev), strconv.FormatUint(n.Id, 10))

			n.sendMessage(netinfo, strconv.FormatUint(NodeId, 10), strconv.FormatUint(oldNext.Id, 10), strconv.FormatUint(oldTwiceNextId, 10), strconv.FormatUint(LeaderId, 10))
			n.state = assimilated
			NetworkState = ring
		}
	} else if n.relation == twicePrev {
		n.state = assimilated
		
		if NetworkState == twoNodes {
			n.sendMessage(requestNextNode)
		}
	} else if n.relation == prev {
		n.state = assimilated
	}	else {
		panic("xxxxxx")
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

	n := &Node{0, none, new, c}
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

func startServer(p uint16, newNetwork bool) {
	l, err := net.Listen("tcp4", fmt.Sprintf(":%d", p))

	if err != nil {
		UserError(err.Error())
		return
	}

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

		n := &Node{0, none, new, c}
		go n.handleClient()
	}
}

func StartNetwork(p uint16) {
	if server != nil {
		UserError("already connected")
		return
	}

	initNode(getIp(), p)
	go startServer(p, true)
}

func JoinNetwork(a string, p uint16) {
	if server != nil {
		UserError("already connected")
		return
	}

	initNode(getIp(), p)
	go startServer(p, false)

	networkNode := Connect(a)

	if networkNode == nil {
		Disconnect()
		UserError("Failed to connect to the remote network")
		return
	}

	networkNode.state = assimilated
	networkNode.relation = prev
	networkNode.sendMessage(connect, strconv.FormatUint(NodeId, 10), strconv.Itoa(next))
}