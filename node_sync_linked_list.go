package main

import "sync"

type nodeSynclinkedListNode struct {
	data *Node
	next *nodeSynclinkedListNode
}

type NodeSyncLinkedList struct {
	head *nodeSynclinkedListNode	
	lock *sync.Mutex
}


func NewNodeSyncLinkedList() *NodeSyncLinkedList {
	return &NodeSyncLinkedList{nil, &sync.Mutex{}}
}

func (l *NodeSyncLinkedList) Add(n *Node) {
	l.lock.Lock()

	newNode := &nodeSynclinkedListNode{n, nil}

	if l.head == nil {
		l.head = newNode
	} else {
		cn := l.head

		for cn.next != nil {
			cn = cn.next
		}

		cn.next = newNode
	}

	l.lock.Unlock()
}

func (l *NodeSyncLinkedList) Remove(id uint64) {
	if l.head == nil {
		return
	}

	l.lock.Lock()

	cn := l.head

	if cn.data.Id == id {
		l.head = cn.next
	} else {
		for cn.next != nil {
			pn := cn
			cn = cn.next

			if cn.data.Id == id {
				pn.next = cn.next
				break
			}
		}
	}

	l.lock.Unlock()
}

func (l *NodeSyncLinkedList) Find(id uint64) *Node {
	cn := l.head

	for cn != nil {
		if cn.data.Id == id {
			return cn.data
		}

		cn = cn.next
	}

	return nil
}

func (l *NodeSyncLinkedList) FindSingleByRelation(r uint8) *Node {
	cn := l.head

	for cn != nil {
		if cn.data.relation == r {
			return cn.data
		}

		cn = cn.next
	}

	return nil
}