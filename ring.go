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

	connect = "con"  				// params=id;relation
	distantNeighborConnect = "dnc" 	// params=id
	//disconnect = "dis" 			// params=
	nextNodeInfoReq = "niq"			// params=id
	nextNodeInfoRes = "nir"			// params=next_id
	netinfo = "nei"					// params=my_id;next_id;leader_id
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
			Log(fmt.Sprintf("DEBUG 0x%X: %s", n.Id, err.Error()))

			n.handleDisconnect()
			return
		}
		data = strings.TrimSpace(data)

		if !n.processMessage(data) {
			n.disconnect()
		}

		AppendChat(data)
	}
}

func (n *Node) disconnect() {
	//n.sendMessage()
	n.connection.Close()
}

func (n *Node) sendMessage(m ...string) {
	msg := magic

	for _, s := range(m) {
		msg += ";" + s
	}
	AppendChat("== " + msg + " ==")
	n.connection.Write([]byte(msg + "\n"))
}

// returns false on failure
func (n *Node) processMessage(m string) bool {
	msg := strings.Split(m, ";")

	if msg[0] != magic {
		return false
	} else {
		switch msg[1] {
			case connect:
				id, err := strconv.ParseUint(msg[2], 10, 64)
				if err != nil {
					return false
				}
				n.Id = id

				rel, err := strconv.ParseUint(msg[3], 10, 8)
				if err != nil {
					return false
				}	
				n.relation = uint8(rel)

				n.handleConnect()

			case netinfo:
				//TODO: handle edge case: two node network 
				nodeId, err := strconv.ParseUint(msg[2], 10, 64)
				if err != nil {
					panic("0")
					return false
				}
				n.Id = nodeId

				nextId, err := strconv.ParseUint(msg[3], 10, 64)
				if err != nil {
					panic("1")
					return false
				}

				leaderId, err := strconv.ParseUint(msg[4], 10, 64)
				if err != nil {
					panic("2")
					return false
				}
				
				nextNode := Connect(IdToEndpoint(nextId))
				if nextNode == nil {
					panic("neinfo failed")
					//return false
				}
				nextNode.state = assimilated
				nextNode.relation = next
				nextNode.Id = nextId
				go nextNode.handleClient()

				 //if nextNode.Id != n.Id {
				nextNode.sendMessage(nextNodeInfoReq, strconv.FormatUint(NodeId, 10))				
				 //} //-- ale pak zustava anonymni
				if leaderId != 0 {
					HandleNewLeader(leaderId)
				}


			case nextNodeInfoReq:
				if n.state == new {
					n.state = assimilated
					n.relation = prev

					id, err := strconv.ParseUint(msg[2], 10, 64)
					if err != nil {
						return false
					}
					n.Id = id
				}

				nextNode := Nodes.FindSingleByRelation(next)
				if nextNode == nil {
					//return false
					panic("invalid state: join requested without next node present")
				}

				n.sendMessage(nextNodeInfoRes, strconv.FormatUint(nextNode.Id, 10))
	
			case nextNodeInfoRes:
				// kontrolovat jestli jsem jeste stale v ringu, mohlo se rozpadnout
				twiceNextNode := Nodes.FindSingleByRelation(twiceNext)
				if twiceNextNode != nil {
					twiceNextNode.disconnect()
					Nodes.Remove(twiceNextNode.Id)
				}

				twiceNextNodeId, err := strconv.ParseUint(msg[2], 10, 64)
				if err != nil {
					return false
				}

				twiceNextNode = Connect(IdToEndpoint(twiceNextNodeId))
				if twiceNextNode == nil {
					//return false
					panic("second neighbor connection failed")
				}
				twiceNextNode.state = assimilated
				twiceNextNode.relation = twiceNext
				twiceNextNode.Id = twiceNextNodeId
				go twiceNextNode.handleClient()
				twiceNextNode.sendMessage(distantNeighborConnect, strconv.FormatUint(NodeId, 10))

				Nodes.Add(twiceNextNode)

			case distantNeighborConnect:
				n.state = assimilated
				n.relation = twicePrev
				id, err := strconv.ParseUint(msg[2], 10, 64)
				if err != nil {
					return false
				}
				n.Id = id
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
			if NetworkState == singleNode {
				n.sendMessage(netinfo, strconv.FormatUint(NodeId, 10), strconv.FormatUint(NodeId, 10), strconv.FormatUint(LeaderId, 10))
				n.relation = next
				n.state = assimilated
				NetworkState = twoNodes
			} else {
				oldNext := Nodes.FindSingleByRelation(next)
				if oldNext == nil {
					panic("invalid network state: twoNodes without next node")
				}

				if NetworkState == ring {
					oldTwiceNext := Nodes.FindSingleByRelation(twiceNext)
					if oldTwiceNext == nil {
						panic("invalid network state: ring without twiceNext node")
					}				
					oldTwiceNext.disconnect()
				}

				oldNext.state = twiceNext

				n.sendMessage(netinfo, strconv.FormatUint(NodeId, 10), strconv.FormatUint(oldNext.Id, 10), strconv.FormatUint(LeaderId, 10))
				n.relation = next
				n.state = assimilated
				NetworkState = ring
			}
	} else if n.relation == follower {

	} else {
		// invalid conn relation
	}
}

func CreateNodeId(i uint32, p uint16, f uint16) {
	//NodeId = uint64(getIp()) << 32 | uint64(p) << 16
	NodeId = uint64(i) << 32 | uint64(p) << 16 | uint64(f)
}

func Connect(a string) *Node {
	c, err := net.Dial("tcp4", a)

	if err != nil {
		UserError(err.Error())
		return nil
	}

	return &Node{0, none, new, c}
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
	if server != nil {
		UserError("already connected")
		return
	}

	l, err := net.Listen("tcp4", fmt.Sprintf(":%d", p))

	if err != nil {
		UserError(err.Error())
		return
	}

	server = l

	if newNetwork {
		NetworkState = singleNode
		UserEvent(fmt.Sprintf("network created, listening on port %d", p))
		Log("NEW NETWORK -> ASSUMING LEADER ROLE")
		HandleNewLeader(NodeId)
	}

	Log(fmt.Sprintf("Server started, listening on port %d. nodeId=0x%X", p, NodeId))
	UpdateStatus()

	// incoming connections
	for server != nil {
		c, err := server.Accept()

		if err != nil {
			Log("DEBUG Server error: " + err.Error())
			return
		}

		n := &Node{0, none, new, c}
		go n.handleClient()
	}
}

func StartNetwork(p uint16) {
	CreateNodeId(getIp(), p, 0)
	Nodes = NewNodeSyncLinkedList()

	go startServer(p, true)
}

func JoinNetwork(a string, p uint16) {
	CreateNodeId(getIp(), p, 0)
	Nodes = NewNodeSyncLinkedList()

	go startServer(p, false)

	networkNode := Connect(a)
	if networkNode == nil {
		UserError("Failed to connect to a remote network")
		return
	}

	networkNode.state = assimilated
	networkNode.relation = prev
	go networkNode.handleClient()
	networkNode.sendMessage(connect, strconv.FormatUint(NodeId, 10), strconv.Itoa(next))
	//networkNode.Id = 0xFAFAFAFAFAFAFAFA
}