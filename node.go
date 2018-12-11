package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Node struct {
	id         uint64
	r          relation
	connection net.Conn
	connected  bool
	lock	   *sync.Mutex
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
	defer updateStatus()
	
	n.lock.Lock()
	n.connected = false
	id := n.id
	r := n.r
	n.lock.Unlock()

	removeNode(n)

	if r == next {
		closeRing(id)

		if id == readLeaderId() || id == getOldLeaderId() {
			if readNetworkState() == ring {
				log("Detected leader node disconnect from r=next")
				updateLeaderId(0)
				setElectionStartTriggerFlag()
				log("Election start trigger flag set")
			}
		}
	} else if r == leader {
		if readNetworkState() == ring {
			log("Leader lost!")
			updateLeaderId(0)
		}
	} else if r == follower {
		removeChatConnection(n)
		log(fmt.Sprintf("Follower lost (id=0x%X), broadcasting updated userlist", n.id))

		msg := []string{userlist}
		msg = append(msg, getConnectedNames()[:]...)
		broadcastToFollowers(msg[:]...)
	}
}

func (n *Node) handleConnect(params []string) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.r == none {
		n.r = next

		oldNext := findNodeByRelationExcludingId(next, n.id)
		
		if oldNext == nil {
			log(fmt.Sprintf("New connection with r=none (id=0x%X), sending netinfo my_id=0x%X, next_id=0x%X (no existing nextnode found), leader_id=0x%X", n.id, nodeId, nodeId, readLeaderId()))
			n.sendMessage(netinfo, idToString(nodeId), idToString(nodeId), idToString(readLeaderId()))
			updateNetworkState(ring)
		} else {
			log(fmt.Sprintf("New next connection while in ring, closing oldNext; old_next_id=0x%X, new_next_id=0x%X", oldNext.id, n.id))
			oldNext.lock.Lock()
			oldNext.r = none
			oldNext.lock.Unlock()
			oldNext.disconnect()

			log(fmt.Sprintf("New connection with r=none (id=0x%X), sending netinfo my_id=0x%X, next_id=0x%X, leader_id=0x%X", n.id, nodeId, oldNext.id, readLeaderId()))
			n.sendMessage(netinfo, idToString(nodeId), idToString(oldNext.id), idToString(readLeaderId()))
		}

	} else if n.r == prev {
	} else if n.r == next {
		atomic.StoreUint32(&ringBroken, 0)

		log("Ring repaired (side missing next)")

		if isElectionStartTriggerFlagSet() {
			log("Detected set election start trigger - starting leader election")
			resetElectionStartTriggerFlag()
			startElectionTimer(0)
		}
	} else if n.r == follower {
		addChatConnection(n, params[0])
		log(fmt.Sprintf("New connection with r=follower (id=0x%X), broadcasting updated userlist", n.id))

		msg := []string{userlist}
		msg = append(msg, getConnectedNames()[:]...)
		broadcastToFollowers(msg[:]...)
	} else {
		panic("invalid connection request")
	}
}

func (n *Node) handleClient() {
	addNode(n)

	log(fmt.Sprintf("New connection (%s -> %s)", n.connection.LocalAddr().String(), n.connection.RemoteAddr().String()))

	r := bufio.NewReader(n.connection)

	for n.connected {
		updateStatus()
		data, err := r.ReadString('\n')

		if err != nil {
			log(fmt.Sprintf("Client 0x%X disconnected, r=%s", n.id, n.r))
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
	msg := strings.Split(m, sepchar)

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

			n.lock.Lock()
			n.id = id
			n.r = relation(msg[parseStartIx+1])
			n.lock.Unlock()

			log(fmt.Sprintf("[%d] Received connect message: remote_id=0x%X, r=%s", messageTime, n.id, n.r))
			n.handleConnect(msg[parseStartIx+2:])

		case netinfo:
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

			remoteLeaderId, err := stringToId(msg[parseStartIx+2])
			if err != nil {
				debugLog("NETINFO remoteLeaderId err")
				return false
			}

			n.lock.Lock()
			n.id = remoteNodeId
			n.lock.Unlock()

			log(fmt.Sprintf("[%d] Received netinfo: remote_id=0x%X, next_id=0x%X, leader_id=0x%X", messageTime, remoteNodeId, nextId, remoteLeaderId))

			log(fmt.Sprintf("Attempting to connect to remote node, id=0x%X", remoteNodeId))
			nextNode := connectToClient(idToEndpoint(nextId))
			if nextNode == nil {
				log(fmt.Sprintf("Connection to remote node failed!, id=0x%X", remoteNodeId))
				log("Will now attempt to close the ring")
				closeRing(nextId)
			} else {
				nextNode.lock.Lock()
				nextNode.r = next
				nextNode.id = nextId
				nextNode.lock.Unlock()
				log(fmt.Sprintf("Connection to remote node was successful, id=0x%X", nextId))

				// notify the next node who we are
				log(fmt.Sprintf("Sending connect message: target_id=0x%X, my_id=0x%X, r=%s", nextNode.id, nodeId, prev))
				nextNode.sendMessage(connect, strconv.FormatUint(nodeId, 16), string(prev))

				// if we have connected to somebody we have a ring
				updateNetworkState(ring)
			}

			// in case there was a leader in the network
			if remoteLeaderId != 0 {
				handleNewLeader(remoteLeaderId)
			}

		case closering:
			senderId, err := stringToId(msg[parseStartIx])
			if err != nil {
				debugLog("CLOSERING sender id failure")
				return false
			}

			log(fmt.Sprintf("[%d] Received closering: from_id=0x%X, sender_id=0x%X", messageTime, n.id, senderId))
			prevNode := findNodeByRelation(prev)

			if prevNode != nil {
				if senderId != nodeId {
					log(fmt.Sprintf("Forwarding closering (from time %d): target_id=0x%X, sender_id=0x%X", messageTime, prevNode.id, senderId))
					prevNode.sendMessage(closering, msg[parseStartIx])
				} else {
					atomic.StoreUint32(&ringBroken, 0)
					log(fmt.Sprintf("Closering propagation stopped (from time %d): target_id=0x%X == sender_id=0x%X", messageTime, prevNode.id, senderId))
				}
			} else {
				log(fmt.Sprintf("[%d] Received closering without having a prevNode! from_id=0x%X, sender_id=0x%X", messageTime, n.id, senderId))
				log(fmt.Sprintf("Sending connect message: target_id=0x%X, my_id=0x%X, r=%s", senderId, nodeId, next))
				prevNode = connectToClient(idToEndpoint(senderId))

				if prevNode == nil {
					log(fmt.Sprintf("Connection to remote node failed!, id=0x%X", senderId))
					return false
				}

				prevNode.lock.Lock()
				prevNode.r = prev
				prevNode.id = senderId
				prevNode.lock.Unlock()
				prevNode.sendMessage(connect, idToString(nodeId), string(next))
				log("Ring repaired (side missing prev)")
			}

		case election:
			candidateId, err := stringToId(msg[parseStartIx])

			if err != nil {
				debugLog("ELECTION candidate id failure")
				return false
			}

			log(fmt.Sprintf("[%d] Received election, from_id=0x%X, candidate_id=0x%X", messageTime, n.id, candidateId))

			nextNode := findNodeByRelation(next)

			if nextNode == nil {
				log(fmt.Sprintf("[%d] No nextnode to forward election to! Discarding.", messageTime))
			} else {
				if candidateId == nodeId {
					log(fmt.Sprintf("[%d] This node has been elected as a new leader! (candidate_id == my_id)", messageTime))

					log(fmt.Sprintf("[%d] Sending elected to target_id=0x%X", messageTime, nextNode.id))					
					nextNode.sendMessage(elected, idToString(nodeId))
					handleNewLeader(nodeId)
				} else if candidateId > nodeId {
					log(fmt.Sprintf("[%d] Forwarding election (candidate_id > my_id), target_id=0x%X, candidate_id=0x%X", messageTime, nextNode.id, candidateId))
					setElectionParticipated()
					
					nextNode.sendMessage(election, idToString(candidateId))
				} else {
					log(fmt.Sprintf("[%d] Discarding election (candidate_id < my_id)", messageTime))

					if !hasElectionParticipated() {
						setElectionParticipated()
						log(fmt.Sprintf("[%d] Sending election, target_id=0x%X, candidate_id=0x%X", messageTime, nextNode.id, nodeId))
						nextNode.sendMessage(election, idToString(nodeId))
					}
				}
			}
			resetElectionTimer()

		case elected:
			newLeaderId, err := stringToId(msg[parseStartIx])

			if err != nil {
				debugLog("ELECTED new leader id failure")
				return false
			}
			log(fmt.Sprintf("[%d] Received elected, from_id=0x%X, leader_id=0x%X", messageTime, n.id, newLeaderId))

			if newLeaderId != nodeId {
				nextNode := findNodeByRelation(next)

				if nextNode != nil {
					log(fmt.Sprintf("[%d] Forwarding elected, target_id=0x%X, leader_id=0x%X", messageTime, nextNode.id, newLeaderId))
					nextNode.sendMessage(elected, idToString(newLeaderId))
				} else {
					log(fmt.Sprintf("[%d] No next node fo forward elected to.", messageTime))
				}

				handleNewLeader(newLeaderId)
			} else {
				log(fmt.Sprintf("[%d] Received elected with leader_id == my_id, stopping propagation, from_id=0x%X, leader_id=0x%X", messageTime, n.id, newLeaderId))
			}

		case chatmessagesend:
			log(fmt.Sprintf("[%d] Received chatmessagesend from_id=0x%X", messageTime, n.id))

			user := getUsername(n)

			chatmsg := []string{chatmessage, user}
			chatmsg = append(chatmsg, msg[parseStartIx:]...)

			log(fmt.Sprintf("Broadcasting chatmessagesend received at %d from_id=0x%X", messageTime, n.id))
			broadcastToFollowers(chatmsg[:]...)

		case chatmessage:
			log(fmt.Sprintf("[%d] Received chatmessage from_id=0x%X", messageTime, n.id))
			user := msg[parseStartIx]
			var chatmsg bytes.Buffer

			for i:=parseStartIx+1; i<len(msg); i++ {
				chatmsg.WriteString(msg[i])
				
				if i+1<len(msg) {
					chatmsg.WriteString(sepchar)
				}
			}

			chatMessageReceived(user, chatmsg.String())

		case userlist:
			log(fmt.Sprintf("[%d] Received userlist from_id=0x%X", messageTime, n.id))
			users := msg[parseStartIx:]
			updateUsers(users)
		}

	}

	return true
}

func nodeFromConnection(c net.Conn) *Node{
	return &Node{0, none, c, true, &sync.Mutex{}}
}

func connectToClient(a string) *Node {
	c, err := net.Dial("tcp4", a)

	if err != nil {
		userError(err.Error())
		return nil
	}

	n := nodeFromConnection(c)
	go n.handleClient()

	return n
}
