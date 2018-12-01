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
	defer l.lock.Unlock()

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
}

func (l *NodeSyncLinkedList) Remove(n *Node) {
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

func (l *NodeSyncLinkedList) FindSingleByRelation(r string) *Node {
	l.lock.Lock()
	defer l.lock.Unlock()

	cn := l.head

	for cn != nil {
		if cn.data.relation == r {
			return cn.data
		}

		cn = cn.next
	}

	return nil
}

func (l *NodeSyncLinkedList) FindSingleByRelationExcludingId(r string, id uint64) *Node {
	l.lock.Lock()
	defer l.lock.Unlock()

	cn := l.head

	for cn != nil {
		if cn.data.relation == r && cn.data.Id != id {
			return cn.data
		}

		cn = cn.next
	}

	return nil
}