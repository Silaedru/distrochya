package main

import "testing"

func TestList(t *testing.T) {
	node1 := &Node{1, 0, 0, nil}
	node2 := &Node{2, 0, 0, nil}
	node3 := &Node{3, 0, 0, nil}
	node4 := &Node{4, 0, 0, nil}

	list := NewNodeSyncLinkedList()

	if list == nil {
		t.FailNow()
	}

	list.Add(node2)
	list.Add(node1)
	list.Add(node3)

	if list.Find(4) != nil {
		t.FailNow()
	}

	if list.Find(3) != node3 {
		t.FailNow()
	}

	if list.Find(1) != node1 {
		t.FailNow()
	}	

	list.Remove(1)

	if list.Find(1) != nil {
		t.FailNow()
	}

	if list.Find(3) != node3 {
		t.FailNow()
	}

	list.Add(node4)

	if list.Find(3) != node3 {
		t.FailNow()
	}

	if list.Find(4) != node4 {
		t.FailNow()
	}

	list.Remove(1)
	list.Remove(2)
	list.Remove(3)
	list.Remove(4)

	if list.head != nil {
		t.FailNow()
	}

	list.Add(node1)
	list.Add(node3)
	list.Add(node2)
	list.Add(node4)

	if list.head.data != node1 {
		t.FailNow()
	}

	list.Remove(4)
	list.Remove(1)
	list.Remove(3)
	list.Remove(2)

	if list.head != nil {
		t.FailNow()
	}
}