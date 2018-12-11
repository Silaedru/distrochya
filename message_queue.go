package main

type messageNode struct {
	m string
	n *messageNode
}

var size uint32 = 0
var messageQueue *messageNode

func enqueueMessage(m string) {
	nn := &messageNode{m, nil}
	size++

	if messageQueue == nil {
		messageQueue = nn
	} else {
		cn := messageQueue

		for cn.n != nil {
			cn = cn.n
		}

		cn.n = nn
	}
}

func clearMessageQueue() []string {
	if size == 0 {
		return nil
	}

	rtn := make([]string, size)

	cn := messageQueue

	for i:=0; cn!=nil; i++ {
		rtn[i] = cn.m
		cn = cn.n
	}

	size = 0
	messageQueue = nil

	return rtn
}