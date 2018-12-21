package main

import "sync"

var chatConnectionsLock = &sync.Mutex{}
var chatConnections = make(map[*Node]string)

func addChatConnection(n *Node, u string) {
	chatConnectionsLock.Lock()
	defer chatConnectionsLock.Unlock()

	chatConnections[n] = u
}

func getConnectedNames() []string {
	chatConnectionsLock.Lock()
	defer chatConnectionsLock.Unlock()

	rtn := make([]string, len(chatConnections))
	i := 0

	for _, n := range chatConnections {
		rtn[i] = n
		i++
	}

	return rtn
}

func getUsername(n *Node) string {
	chatConnectionsLock.Lock()
	defer chatConnectionsLock.Unlock()

	return chatConnections[n]
}

func removeChatConnection(n *Node) {
	chatConnectionsLock.Lock()
	defer chatConnectionsLock.Unlock()

	delete(chatConnections, n)
}

func resetChatConnections() {
	chatConnectionsLock.Lock()
	defer chatConnectionsLock.Unlock()

	chatConnections = make(map[*Node]string)
}
