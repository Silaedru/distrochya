package main

import "sync"

type nodeSyncLinkedListNode struct {
	data *Node
	next *nodeSyncLinkedListNode
}

type nodeSyncLinkedList struct {
	head *nodeSyncLinkedListNode
	lock *sync.Mutex
	size uint32
}

func newNodeSyncLinkedList() *nodeSyncLinkedList {
	return &nodeSyncLinkedList{nil, &sync.Mutex{}, 0}
}

func (l *nodeSyncLinkedList) add(n *Node) {
	l.lock.Lock()
	defer l.lock.Unlock()

	newNode := &nodeSyncLinkedListNode{n, l.head}

	l.head = newNode
	
	l.size++
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
		l.size--
	} else {
		for cn.next != nil {
			pn := cn
			cn = cn.next

			if cn.data == n {
				pn.next = cn.next
				l.size--
				return
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

func (l *nodeSyncLinkedList) findSingleByRelationExcludingID(r relation, id uint64) *Node {
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

func (l *nodeSyncLinkedList) toSlice() []*Node {
	l.lock.Lock()
	defer l.lock.Unlock()

	rtn := make([]*Node, l.size)
	i := 0
	cn := l.head

	for cn != nil {
		rtn[i] = cn.data
		cn = cn.next
		i++
	}

	return rtn
}
