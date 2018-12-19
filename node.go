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
	lock       *sync.Mutex
	kat        *time.Timer
	katLock    *sync.Mutex
}

func (n *Node) disconnect() {
	n.connection.SetDeadline(time.Now())
	n.connection.Close()
}

func (n *Node) sendMessage(m ...string) {
	msg := fmt.Sprintf("%s%s%d", magic, sepchar, advanceTime())

	for _, s := range m {
		msg += sepchar + s
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

	if n.kat != nil {
		n.kat.Stop()
	}

	n.lock.Unlock()

	removeNode(n)

	if r == next {
		closeRing(id)

		if id == getLeaderID() || id == getOldLeaderID() {
			if getNetworkState() == ring {
				log("Detected leader node disconnect from r=next")
				updateLeaderID(0)
				setElectionStartTriggerFlag()
				log("Election start trigger flag set")
			}
		}
	} else if r == leader {
		if getNetworkState() == ring {
			log("Leader lost!")
			updateLeaderID(0)
		}
	} else if r == follower {
		removeChatConnection(n)
		log(fmt.Sprintf("Follower lost (id=0x%X), broadcasting updated userlist", n.id))

		msg := []string{userlist}
		msg = append(msg, getConnectedNames()[:]...)
		broadcastToFollowers(msg[:]...)
	}
}

func (n *Node) processConnectMessage(params []string) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.r == none {
		n.r = next

		oldNext := findNodeByRelationExcludingID(next, n.id)

		if oldNext == nil {
			log(fmt.Sprintf("New connection with r=none (id=0x%X), sending netinfo my_id=0x%X, next_id=0x%X (no existing nextnode found), leader_id=0x%X, twice_next_node_id=0x%X", n.id, nodeID, nodeID, getLeaderID(), n.id))
			n.sendMessage(netinfo, idToString(nodeID), idToString(nodeID), idToString(getLeaderID()), idToString(n.id))
			updateNetworkState(ring)
			updateTwiceNextNodeID(nodeID)
		} else {
			log(fmt.Sprintf("New next connection while in ring, closing oldNext; old_next_id=0x%X, new_next_id=0x%X", oldNext.id, n.id))
			oldTwiceNextNodeID := getTwiceNextNodeID()
			oldNext.lock.Lock()
			oldNext.r = none
			updateTwiceNextNodeID(oldNext.id)
			oldNext.lock.Unlock()
			oldNext.disconnect()

			log(fmt.Sprintf("New connection with r=none (id=0x%X), sending netinfo my_id=0x%X, next_id=0x%X, leader_id=0x%X, twice_next_node_id=0x%X", n.id, nodeID, oldNext.id, getLeaderID(), oldTwiceNextNodeID))
			n.sendMessage(netinfo, idToString(nodeID), idToString(oldNext.id), idToString(getLeaderID()), idToString(oldTwiceNextNodeID))
		}

		prevNode := findNodeByRelation(prev)

		if prevNode != nil {
			prevNode.lock.Lock()
			log(fmt.Sprintf("New next connection, sending nextinfo to my prev, target_id=0x%X, next_id=0x%X", prevNode.id, n.id))
			prevNode.lock.Unlock()
			prevNode.sendMessage(nextinfo, idToString(n.id))
		}
	} else if n.r == prev {
	} else if n.r == next {
		atomic.StoreUint32(&ringBroken, 0)

		log("Ring repaired (side missing next)")

		prevNode := findNodeByRelation(prev)
		if prevNode == nil {
			panic("ring repaired (side missing next) without having prevNode")
		}
		log(fmt.Sprintf("Ring repaired (new next), sending nextinfo to my prev, target_id=0x%X, next_id=0x%X", prevNode.id, n.id))
		prevNode.sendMessage(nextinfo, idToString(n.id))

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
		n.lock.Unlock()
		broadcastToFollowers(msg[:]...)
		n.lock.Lock()
	} else {
		panic("invalid connection request")
	}
}

func (n *Node) handleConnection() {
	var zeroTime time.Time

	addNode(n)

	log(fmt.Sprintf("New connection (%s -> %s)", n.connection.LocalAddr().String(), n.connection.RemoteAddr().String()))

	r := bufio.NewReader(n.connection)

	for n.connected {
		updateStatus()

		n.resetKeepAliveTimer()
		n.connection.SetReadDeadline(time.Now().Add((connectionTimeoutSeconds + connectionTimeoutGraceSeconds) * time.Second))
		data, err := r.ReadString('\n')
		n.connection.SetReadDeadline(zeroTime)
		n.katLock.Lock()
		n.kat.Stop()
		n.katLock.Unlock()

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

func (n *Node) keepAlive() {
	n.lock.Lock()

	if n.connected && (n.r == next || n.r == leader) {
		n.lock.Unlock()
		log(fmt.Sprintf("Sending alivecheck (PING), target_id=0x%X", n.id))
		n.sendMessage(alivecheck)
	} else {
		n.lock.Unlock()
	}

	n.resetKeepAliveTimer()
}

func (n *Node) resetKeepAliveTimer() {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.katLock.Lock()
	defer n.katLock.Unlock()

	if n.kat != nil {
		n.kat.Stop()
	}

	if n.connected {
		n.kat = time.AfterFunc(connectionTimeoutSeconds/3*time.Second, n.keepAlive)
	}
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
			id, err := stringToID(msg[parseStartIx])
			if err != nil {
				debugLog("CONNECT id err")
				return false
			}

			n.lock.Lock()
			n.id = id
			n.r = relation(msg[parseStartIx+1])
			n.lock.Unlock()

			log(fmt.Sprintf("[%d] Received connect message: remote_id=0x%X, r=%s", messageTime, n.id, n.r))
			n.processConnectMessage(msg[parseStartIx+2:])

		case netinfo:
			remoteNodeID, err := stringToID(msg[parseStartIx])
			if err != nil {
				debugLog("NETINFO remoteNodeID err")
				return false
			}

			nextID, err := stringToID(msg[parseStartIx+1])
			if err != nil {
				debugLog("NETINFO nextID err")
				return false
			}

			remoteLeaderID, err := stringToID(msg[parseStartIx+2])
			if err != nil {
				debugLog("NETINFO remoteLeaderID err")
				return false
			}

			remoteTwiceNextNodeID, err := stringToID(msg[parseStartIx+3])
			if err != nil {
				debugLog("NETINFO remoteTwiceNextNodeID err")
				return false
			}

			n.lock.Lock()
			n.id = remoteNodeID
			n.lock.Unlock()

			updateTwiceNextNodeID(remoteTwiceNextNodeID)

			log(fmt.Sprintf("[%d] Received netinfo: remote_id=0x%X, next_id=0x%X, leader_id=0x%X, twice_next_node_id=0x%X", messageTime, remoteNodeID, nextID, remoteLeaderID, remoteTwiceNextNodeID))

			log(fmt.Sprintf("Attempting to connect to remote node, id=0x%X", remoteNodeID))
			nextNode := connectToNode(idToEndpoint(nextID))
			if nextNode == nil {
				log(fmt.Sprintf("Connection to remote node failed!, id=0x%X", remoteNodeID))
				log("Will now attempt to close the ring")
				closeRing(nextID)
			} else {
				nextNode.lock.Lock()
				nextNode.r = next
				nextNode.id = nextID
				nextNode.lock.Unlock()
				log(fmt.Sprintf("Connection to remote node was successful, id=0x%X", nextID))

				// notify the next node who we are
				log(fmt.Sprintf("Sending connect message: target_id=0x%X, my_id=0x%X, r=%s", nextNode.id, nodeID, prev))
				nextNode.sendMessage(connect, strconv.FormatUint(nodeID, 16), string(prev))

				// if we have connected to somebody we have a ring
				updateNetworkState(ring)
			}

			// in case there was a leader in the network
			if remoteLeaderID != 0 {
				handleNewLeader(remoteLeaderID)
			}

		case closering:
			senderID, err := stringToID(msg[parseStartIx])
			if err != nil {
				debugLog("CLOSERING sender id failure")
				return false
			}

			log(fmt.Sprintf("[%d] Received closering: from_id=0x%X, sender_id=0x%X", messageTime, n.id, senderID))
			prevNode := findNodeByRelation(prev)

			if prevNode != nil {
				if senderID != nodeID {
					log(fmt.Sprintf("Forwarding closering (from time %d): target_id=0x%X, sender_id=0x%X", messageTime, prevNode.id, senderID))
					prevNode.sendMessage(closering, msg[parseStartIx])
				} else {
					atomic.StoreUint32(&ringBroken, 0)
					log(fmt.Sprintf("Closering propagation stopped (from time %d): target_id=0x%X == sender_id=0x%X", messageTime, prevNode.id, senderID))
				}
			} else {
				log(fmt.Sprintf("[%d] Received closering without having a prevNode! from_id=0x%X, sender_id=0x%X", messageTime, n.id, senderID))
				log(fmt.Sprintf("Sending connect message: target_id=0x%X, my_id=0x%X, r=%s", senderID, nodeID, next))

				if n.r == none {
					prevNode = n
				} else {
					prevNode = connectToNode(idToEndpoint(senderID))
				}

				if prevNode == nil {
					log(fmt.Sprintf("Connection to remote node failed!, id=0x%X", senderID))
					return false
				}

				prevNode.lock.Lock()
				prevNode.r = prev
				prevNode.id = senderID
				prevNode.lock.Unlock()
				prevNode.sendMessage(connect, idToString(nodeID), string(next))
				log("Ring repaired (side missing prev)")

				nextNode := findNodeByRelation(next)
				if nextNode == nil {
					panic("ring repaired (side missing prev) without having nextNode")
				}
				log(fmt.Sprintf("Ring repaired, sending nextinfo to my new prev, target_id=0x%X, next_id=0x%X", prevNode.id, nextNode.id))
				prevNode.sendMessage(nextinfo, idToString(nextNode.id))
			}

		case election:
			candidateID, err := stringToID(msg[parseStartIx])

			if err != nil {
				debugLog("ELECTION candidate id failure")
				return false
			}

			if getLeaderID() != 0 {
				log("New election detected, removing currently elected leader")
				updateLeaderID(0)
			}

			log(fmt.Sprintf("[%d] Received election, from_id=0x%X, candidate_id=0x%X", messageTime, n.id, candidateID))

			nextNode := findNodeByRelation(next)

			if nextNode == nil {
				log(fmt.Sprintf("[%d] No nextnode to forward election to! Discarding.", messageTime))
			} else {
				if candidateID == nodeID {
					log(fmt.Sprintf("[%d] This node has been elected as a new leader! (candidate_id == my_id)", messageTime))

					log(fmt.Sprintf("[%d] Sending elected to target_id=0x%X", messageTime, nextNode.id))
					nextNode.sendMessage(elected, idToString(nodeID))
					handleNewLeader(nodeID)
				} else if candidateID > nodeID {
					log(fmt.Sprintf("[%d] Forwarding election (candidate_id > my_id), target_id=0x%X, candidate_id=0x%X", messageTime, nextNode.id, candidateID))
					setElectionParticipated()

					nextNode.sendMessage(election, idToString(candidateID))
				} else {
					log(fmt.Sprintf("[%d] Discarding election (candidate_id < my_id)", messageTime))

					if !hasElectionParticipated() {
						setElectionParticipated()
						log(fmt.Sprintf("[%d] Sending election, target_id=0x%X, candidate_id=0x%X", messageTime, nextNode.id, nodeID))
						nextNode.sendMessage(election, idToString(nodeID))
					}
				}
			}
			resetElectionTimer()

		case elected:
			newLeaderID, err := stringToID(msg[parseStartIx])

			if err != nil {
				debugLog("ELECTED new leader id failure")
				return false
			}
			log(fmt.Sprintf("[%d] Received elected, from_id=0x%X, leader_id=0x%X", messageTime, n.id, newLeaderID))

			if newLeaderID != nodeID {
				nextNode := findNodeByRelation(next)

				if nextNode != nil {
					log(fmt.Sprintf("[%d] Forwarding elected, target_id=0x%X, leader_id=0x%X", messageTime, nextNode.id, newLeaderID))
					nextNode.sendMessage(elected, idToString(newLeaderID))
				} else {
					log(fmt.Sprintf("[%d] No next node fo forward elected to.", messageTime))
				}

				handleNewLeader(newLeaderID)
			} else {
				log(fmt.Sprintf("[%d] Received elected with leader_id == my_id, stopping propagation, from_id=0x%X, leader_id=0x%X", messageTime, n.id, newLeaderID))
			}

		case chatmessagesend:
			log(fmt.Sprintf("[%d] Received chatmessagesend, from_id=0x%X", messageTime, n.id))

			user := getUsername(n)

			chatmsg := []string{chatmessage, user}
			chatmsg = append(chatmsg, msg[parseStartIx:]...)

			log(fmt.Sprintf("Broadcasting chatmessagesend received at %d, from_id=0x%X", messageTime, n.id))
			broadcastToFollowers(chatmsg[:]...)

		case chatmessage:
			log(fmt.Sprintf("[%d] Received chatmessage, from_id=0x%X", messageTime, n.id))
			user := msg[parseStartIx]
			var chatmsg bytes.Buffer

			for i := parseStartIx + 1; i < len(msg); i++ {
				chatmsg.WriteString(msg[i])

				if i+1 < len(msg) {
					chatmsg.WriteString(sepchar)
				}
			}

			chatMessageReceived(user, chatmsg.String())

		case userlist:
			log(fmt.Sprintf("[%d] Received userlist, from_id=0x%X", messageTime, n.id))
			users := msg[parseStartIx:]
			updateUsers(users)

		case nextinfo:
			newTwiceNextNodeID, err := stringToID(msg[parseStartIx])

			if err != nil {
				debugLog("NEXTINFO new twice next node id failure")
				return false
			}
			log(fmt.Sprintf("[%d] Received nextinfo, from_id=0x%X, twice_next_node_id=0x%X", messageTime, n.id, newTwiceNextNodeID))
			updateTwiceNextNodeID(newTwiceNextNodeID)

		case alivecheck:
			log(fmt.Sprintf("[%d] Received alivecheck (PING), from_id=0x%X", messageTime, n.id))
			log(fmt.Sprintf("Sending aliveresponse (PONG) (alivecheck from %d), target_id=0x%X", messageTime, n.id))
			n.sendMessage(aliveresponse)

		case aliveresponse:
			log(fmt.Sprintf("[%d] Received aliveresponse (PONG), from_id=0x%X", messageTime, n.id))
		}

	}

	return true
}

func nodeFromConnection(c net.Conn) *Node {
	return &Node{0, none, c, true, &sync.Mutex{}, nil, &sync.Mutex{}}
}

func connectToNode(a string) *Node {
	c, err := net.Dial("tcp4", a)

	if err != nil {
		userError(err.Error())
		return nil
	}

	n := nodeFromConnection(c)
	go n.handleConnection()

	return n
}
