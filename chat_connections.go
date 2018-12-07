package main

import "sync"

type chatConnection struct {
	n *Node
	u string
}

type chatConnectionLinkedListNode struct {
	data *chatConnection 
	next *chatConnectionLinkedListNode
}

type chatConnectionLinkedList struct {
	head *chatConnectionLinkedListNode
	lock *sync.Mutex
	size uint32
}

var chatConnections = &chatConnectionLinkedList{nil, &sync.Mutex{}, 0}

func addChatConnection(n *Node, u string) {
	chatConnections.lock.Lock()
	defer chatConnections.lock.Unlock()

	newNode := &chatConnectionLinkedListNode{&chatConnection{n, u}, nil}

	if chatConnections.head == nil {
		chatConnections.head = newNode
	} else {
		cn := chatConnections.head

		for cn.next != nil {
			cn = cn.next
		}

		cn.next = newNode
	}

	chatConnections.size++
}

func getConnectedNames() []string {
	chatConnections.lock.Lock()
	defer chatConnections.lock.Unlock()

	rtn := make([]string, chatConnections.size)
	i := 0
	cn := chatConnections.head

	for cn != nil {
		rtn[i] = cn.data.u
		cn = cn.next
		i++
	}

	return rtn
}

func getUsername(n *Node) string {
	chatConnections.lock.Lock()
	defer chatConnections.lock.Unlock()

	cn := chatConnections.head

	for cn != nil {
		if cn.data.n == n {
			return cn.data.u
		}

		cn = cn.next
	}

	return ""
}

func removeChatConnection(n* Node) {
	chatConnections.lock.Lock()
	defer chatConnections.lock.Unlock()

	if chatConnections.head == nil {
		return
	}

	cn := chatConnections.head

	if cn.data.n == n {
		chatConnections.head = cn.next
		chatConnections.size--
	} else {
		for cn.next != nil {
			pn := cn
			cn = cn.next

			if cn.data.n == n {
				pn.next = cn.next
				chatConnections.size--
				return
			}
		}
	}
}

func resetChatConnections() {
	chatConnections.lock.Lock()
	defer chatConnections.lock.Unlock()
	chatConnections.size = 0
	chatConnections.head = nil
}
