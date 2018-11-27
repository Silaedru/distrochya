package main

import (
	"net"
	"fmt"
	"bufio"
	"bytes"
	"strings"
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
	unknown = iota

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
	none = iota
	singleNode
	twoNodes
	ring
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
	//defer n.connection.Close()
	Nodes.Add(n)
	Log("New client connected")

	r := bufio.NewReader(n.connection)

	for {
		UpdateStatus()
		data, err := r.ReadString('\n')

		if err != nil {
			Log(fmt.Sprintf("Client 0x%X: connection lost", n.Id))
			Log(fmt.Sprintf("DEBUG 0x%X: %s", n.Id, err.Error()))

			Nodes.Remove(n.Id)
			UpdateStatus()
			return
		}
		data = strings.TrimSpace(data)


		/*
		result := strconv.Itoa(random()) + "\n"
        c.Write([]byte(string(result)))*/

		AppendChat(data)
	}
}

func (n *Node) disconnect() {
	n.connection.Close()
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

func StartNetwork(p uint16) {
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
	NodeId = uint64(getIp()) << 32 | uint64(p) << 16
	NetworkState = singleNode
	Nodes = NewNodeSyncLinkedList()

	Log(fmt.Sprintf("Server started, listening on port %d. nodeId=0x%X", p, NodeId))
	UserEvent(fmt.Sprintf("network started, listening on port %d", p))

	Log("NEW NETWORK -> ASSUMING LEADER ROLE")
	LeaderId = NodeId
	HandleNewLeader(LeaderId)

	// incoming connections
	for server != nil {
		c, err := server.Accept()

		if err != nil {
			//Log("Server error: " + err.Error())
			return
		}

		n := &Node{0, unknown, new, c}
		go n.handleClient()
	}
}

func JoinNetwork() {
	if server != nil {
		UserError("already connected")
		return
	}
}