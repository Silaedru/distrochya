package main

import "sync"

type nodeSyncLinkedListNode struct {
	data *Node
	next *nodeSyncLinkedListNode
}

type nodeSyncLinkedList struct {
	head *nodeSyncLinkedListNode
	lock *sync.Mutex
}

func newNodeSyncLinkedList() *nodeSyncLinkedList {
	return &nodeSyncLinkedList{nil, &sync.Mutex{}}
}

func (l *nodeSyncLinkedList) add(n *Node) {
	l.lock.Lock()
	defer l.lock.Unlock()

	newNode := &nodeSyncLinkedListNode{n, nil}

	if l.head == nil {
		l.head = newNode
	} else {
		cn := l.head

		for cn.next != nil {
			cn = cn.next
		}

		cn.next = newNode
	}
}

func (l *nodeSyncLinkedList) remove(n *Node) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.head == nil {
		return
	}

	cn := l.head

	if cn.data == n {
		l.head = cn.next
	} else {
		for cn.next != nil {
			pn := cn
			cn = cn.next

			if cn.data == n {
				pn.next = cn.next
				break
			}
		}
	}
}

func (l *nodeSyncLinkedList) findSingleByRelation(r relation) *Node {
	l.lock.Lock()
	defer l.lock.Unlock()

	cn := l.head

	for cn != nil {
		if cn.data.r == r {
			return cn.data
		}

		cn = cn.next
	}

	return nil
}

func (l *nodeSyncLinkedList) findSingleByRelationExcludingId(r relation, id uint64) *Node {
	l.lock.Lock()
	defer l.lock.Unlock()

	cn := l.head

	for cn != nil {
		if cn.data.r == r && cn.data.id != id {
			return cn.data
		}

		cn = cn.next
	}

	return nil
}
