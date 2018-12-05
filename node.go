package main

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type Node struct {
	id         uint64
	relation   string
	connection net.Conn
	connected  bool
}

func (n *Node) disconnect() {
	n.connection.SetDeadline(time.Now())
	n.connection.Close()
}

func (n *Node) sendMessage(m ...string) {
	msg := fmt.Sprintf("%s;%d", magic, advanceTime())

	for _, s := range m {
		msg += ";" + s
	}

	debugLog("SEND: ==" + msg + "== (" + idToString(n.id) + ")")
	n.connection.SetWriteDeadline(time.Now().Add(sendMessageTimeoutSeconds * time.Second))
	_, err := n.connection.Write([]byte(msg + "\n"))

	if err != nil {
		debugLog(fmt.Sprintf("WRITE ERR: 0x%X: %s", n.id, err.Error()))
		n.disconnect()
	}
	var zeroTime time.Time
	n.connection.SetWriteDeadline(zeroTime)
}

func (n *Node) handleDisconnect() {
	n.connected = false
	nodes.remove(n)

	// connection to next node lost
	if n.relation == next {
		closeRing(n.id)
	}

	updateStatus()
}

func (n *Node) handleConnect() {
	if n.relation == none {
		n.relation = next

		// new node is joining the network
		if readNetworkState() == singleNode {
			n.sendMessage(netinfo, idToString(nodeId), idToString(nodeId), idToString(leaderId))
			updateNetworkState(ring)
		} else {
			oldNext := nodes.findSingleByRelationExcludingId(next, n.id)
			if oldNext == nil {
				panic("invalid network state: ring without next node")
			}
			log(fmt.Sprintf("New next connection while in ring, closing oldNext; old_next_id=0x%X, new_next_id=0x%X", oldNext.id, n.id))
			oldNext.relation = none
			oldNext.disconnect()

			n.sendMessage(netinfo, idToString(nodeId), idToString(oldNext.id), idToString(leaderId))
		}
	} else if n.relation == prev {
	} else if n.relation == next {
		atomic.StoreUint32(&ringBroken, 0)
	} else {
		panic("invalid connection request")
	}
}

func (n *Node) handleClient() {
	n.connected = true
	nodes.add(n)
	log("New connection!")

	r := bufio.NewReader(n.connection)

	for n.connected {
		updateStatus()
		data, err := r.ReadString('\n')

		if err != nil {
			log(fmt.Sprintf("Client 0x%X disconnected, relation=%s", n.id, n.relation))
			debugLog(fmt.Sprintf("READ ERR: 0x%X: %s", n.id, err.Error()))

			n.handleDisconnect()
			return
		}
		data = strings.TrimSpace(data)

		if !n.processMessage(data) {
			n.disconnect()
			log(fmt.Sprintf("Invalid message from client 0x%X, disconnecting", n.id))
			log("IMSG: " + data)
		}

		debugLog("RECV: ==" + data + "== (" + idToString(n.id) + ")")
	}

	n.handleDisconnect()
}

// returns false on failure
func (n *Node) processMessage(m string) bool {
	msg := strings.Split(m, ";")

	if len(msg) < 3 || msg[0] != magic {
		return false
	} else {
		recvdTime, err := strconv.ParseUint(msg[1], 10, 64)

		if err != nil {
			debugLog("processMessage timestamp parse failure")
			return false
		}

		messageTime := updateTime(recvdTime)
		parseStartIx := 3

		switch msg[2] {

		// node would like to connect
		case connect:
			id, err := stringToId(msg[parseStartIx])
			if err != nil {
				debugLog("CONNECT id err")
				return false
			}

			n.id = id
			n.relation = msg[parseStartIx+1]

			log(fmt.Sprintf("[%d] Received connect message: remote_id=0x%X, relation=%s", messageTime, n.id, n.relation))
			n.handleConnect()

		// response to initial connect request containing basic network info
		case netinfo:
			// will happen when response to closering connection is received
			remoteNodeId, err := stringToId(msg[parseStartIx])
			if err != nil {
				debugLog("NETINFO remoteNodeId err")
				return false
			}

			nextId, err := stringToId(msg[parseStartIx+1])
			if err != nil {
				debugLog("NETINFO nextId err")
				return false
			}

			leaderId, err := stringToId(msg[parseStartIx+2])
			if err != nil {
				debugLog("NETINFO leaderId err")
				return false
			}

			n.id = remoteNodeId
			log(fmt.Sprintf("[%d] Received netinfo: remote_id=0x%X, next_id=0x%X, leader_id=0x%X", messageTime, remoteNodeId, nextId, leaderId))

			log(fmt.Sprintf("Attempting to connect to remote node, id=0x%X", remoteNodeId))
			nextNode := connectToClient(idToEndpoint(nextId))
			if nextNode == nil {
				log(fmt.Sprintf("Connection to remote node failed!, id=0x%X", remoteNodeId))
				log("Will now attempt to close the ring")
				closeRing(nextId)
				//return false
			} else {
				nextNode.relation = next
				nextNode.id = nextId
				log(fmt.Sprintf("Connection to remote node was successful, id=0x%X", nextId))

				// notify the next node who we are
				log(fmt.Sprintf("Sending connect message: target_id=0x%X, my_id=0x%X, relation=%s", nextNode.id, nodeId, prev))
				nextNode.sendMessage(connect, strconv.FormatUint(nodeId, 16), prev)

				// if we have connected to somebody we have a ring
				updateNetworkState(ring)
			}
			// in case there was a leader in the network
			if leaderId != 0 {
				handleNewLeader(leaderId)
			}

		case closering:
			senderId, err := stringToId(msg[parseStartIx])
			if err != nil {
				debugLog("CLOSERING sender id failure")
				return false
			}

			log(fmt.Sprintf("Received closering: from_id=0x%X, sender_id=0x%X", n.id, senderId))
			prevNode := nodes.findSingleByRelation(prev)

			if prevNode != nil {
				if senderId != nodeId {
					log(fmt.Sprintf("Forwarding closering: target_id=0x%X, sender_id=0x%X", prevNode.id, senderId))
					prevNode.sendMessage(closering, msg[parseStartIx])
				} else {
					atomic.StoreUint32(&ringBroken, 0)
					log(fmt.Sprintf("Closering propagation stopped: target_id=0x%X == sender_id=0x%X", prevNode.id, senderId))
				}
			} else {
				log(fmt.Sprintf("Received closering without having a prevNode! from_id=0x%X, sender_id=0x%X", n.id, senderId))
				log(fmt.Sprintf("Sending connect message: target_id=0x%X, my_id=0x%X, relation=%s", senderId, nodeId, next))
				prevNode = connectToClient(idToEndpoint(senderId))

				if prevNode == nil {
					log(fmt.Sprintf("Connection to remote node failed!, id=0x%X", senderId))
					return false
				}

				prevNode.relation = prev
				prevNode.id = senderId
				prevNode.sendMessage(connect, idToString(nodeId), next)
			}
		}
	}

	return true
}

func connectToClient(a string) *Node {
	c, err := net.Dial("tcp4", a)

	if err != nil {
		userError(err.Error())
		return nil
	}

	n := &Node{0, none, c, false}
	go n.handleClient()

	return n
}
