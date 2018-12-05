package main

import (
	"fmt"
	"github.com/jroimartin/gocui"
	"strings"
)

const (
	logViewName       = "log"
	statusViewName    = "status"
	chatViewName      = "chat"
	usersViewName     = "users"
	chatInputViewName = "chatInput"
	controlsViewName  = "controls"

	logViewTitle    = "log"
	statusViewTitle = "Status"
	chatViewTitle   = "Chat"
	usersViewTitle  = "Users"
)

type chatInput struct {
}

var gui *gocui.Gui

func (e *chatInput) onEnter(v *gocui.View) {
	input := strings.TrimSpace(v.Buffer())

	clearView(chatInputViewName)
	v.SetCursor(0, 0)
	v.SetOrigin(0, 0)

	if len(input) > 1 && input[0] == '/' {
		args := strings.Split(input, " ")
		processCommand(args[0], args[1:])
	} else {
		chatMessage(input)
	}
}

func (e *chatInput) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case key == gocui.KeyEnter:
		e.onEnter(v)
	case key == gocui.KeySpace:
		v.EditWrite(' ')

	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		v.EditDelete(true)
	case key == gocui.KeyDelete:
		v.EditDelete(false)
	case key == gocui.KeyArrowLeft:
		v.MoveCursor(-1, 0, false)
	case key == gocui.KeyArrowRight:
		v.MoveCursor(1, 0, false)

	case key == gocui.KeyHome:
		v.SetCursor(0, 0)
		v.SetOrigin(0, 0)
	case key == gocui.KeyArrowUp:
		v.SetCursor(0, 0)
		v.SetOrigin(0, 0)

	case key == gocui.KeyArrowDown:
		v.MoveCursor(len(v.Buffer())-1, 0, false)
	case key == gocui.KeyEnd:
		v.MoveCursor(len(v.Buffer())-1, 0, false)

	case key == gocui.KeyF1:
		return
	case key == gocui.KeyF10:
		return

	case ch != 0 && mod == 0:
		v.EditWrite(ch)
	}
}

func clearView(n string) {
	gui.Update(func(g *gocui.Gui) error {
		view, err := g.View(n)

		if err != nil {
			panic(err)
		}

		view.Clear()
		view.SetCursor(0, 0)
		view.SetOrigin(0, 0)

		return nil
	})
}

func appendView(n string, s string) {
	gui.Update(func(g *gocui.Gui) error {
		view, err := gui.View(n)

		if err != nil {
			panic(err)
		}

		fmt.Fprint(view, s)

		return nil
	})
}

func overwriteView(n string, s string) {
	gui.Update(func(g *gocui.Gui) error {
		view, err := gui.View(n)

		if err != nil {
			panic(err)
		}

		view.Clear()
		view.SetCursor(0, 0)
		view.SetOrigin(0, 0)
		fmt.Fprint(view, s)

		return nil
	})
}

func updateUsers() {
}

func updateStatus() {
	if !debugEnabled {
		return
	}

	debugLog("updateStatus")

	var nodesStr string
	if nodes != nil {
		nodes.lock.Lock()
		cn := nodes.head
		for cn != nil {
			nodesStr = fmt.Sprintf("%s\n   -> 0x%X (listening on %s): %s", nodesStr, cn.data.id, idToEndpoint(cn.data.id), cn.data.relation)
			cn = cn.next
		}
		nodes.lock.Unlock()
	}

	overwriteView(statusViewName, fmt.Sprintf(""+
		"Logical time: %d\n"+
		"Network state: %s\n"+
		"nodeId:   0x%X (%s)\n"+
		"leaderId: 0x%X (%s)\n"+
		"\n"+
		"Connected nodes:\n%s\n\nEND", readTime(), readNetworkState(), nodeId, idToEndpoint(nodeId), leaderId, idToEndpoint(leaderId), nodesStr))
}

func appendLogView(s string) {
	if !debugEnabled {
		return
	}

	appendView(logViewName, s+"\n")
}

func appendChatView(s string) {
	appendView(chatViewName, s+"\n")
}

func layout(g *gocui.Gui) error {
	maxW, maxH := g.Size()

	maxW -= 1
	maxH -= 1

	curY := 0

	if debugEnabled {
		logViewWidth := maxW * 3 / 5
		logViewHeight := maxH / 2

		logView, err := g.SetView(logViewName, 0, curY, logViewWidth, logViewHeight)

		if err != nil && err != gocui.ErrUnknownView {
			panic(err)
		}

		logView.Wrap = true
		logView.Title = logViewTitle
		logView.Autoscroll = true

		statusView, err := g.SetView(statusViewName, logViewWidth+1, curY, maxW, logViewHeight)

		if err != nil && err != gocui.ErrUnknownView {
			panic(err)
		}

		statusView.Wrap = true
		statusView.Title = statusViewTitle

		curY = logViewHeight + 1
	}

	chatViewWidth := maxW * 4 / 5

	chatView, err := g.SetView(chatViewName, 0, curY, chatViewWidth, maxH-3)

	if err != nil && err != gocui.ErrUnknownView {
		panic(err)
	}

	chatView.Wrap = true
	chatView.Autoscroll = true
	chatView.Title = chatViewTitle

	usersView, err := g.SetView(usersViewName, chatViewWidth+1, curY, maxW, maxH-2)

	if err != nil && err != gocui.ErrUnknownView {
		panic(err)
	}

	usersView.Wrap = false
	usersView.Title = usersViewTitle

	chatInputView, err := g.SetView(chatInputViewName, 0, maxH-4, chatViewWidth, maxH-2)

	if err != nil && err != gocui.ErrUnknownView {
		panic(err)
	}

	chatInputView.Editable = true
	chatInputView.Wrap = false
	chatInputView.Editor = &chatInput{}

	controlsView, err := g.SetView(controlsViewName, 0, maxH-2, maxW, maxH)

	if err != nil && err != gocui.ErrUnknownView {
		panic(err)
	}

	controlsView.Wrap = false
	controlsView.Frame = false

	g.SetCurrentView(chatInputViewName)

	return nil
}

func setupKeyBindings(g *gocui.Gui) {
	g.SetKeybinding("", gocui.KeyPgup, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		clearView(chatViewName)
		return nil
	})

	g.SetKeybinding("", gocui.KeyPgdn, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return nil
	})

	g.SetKeybinding("", gocui.KeyF1, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		processCommand("/help", nil)
		return nil
	})

	g.SetKeybinding("", gocui.KeyF10, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return gocui.ErrQuit
	})
}

func InitializeTui() {
	var err error
	gui, err = gocui.NewGui(gocui.OutputNormal)

	if err != nil {
		panic(err)
	}

	defer gui.Close()

	gui.SetManagerFunc(layout)
	setupKeyBindings(gui)

	gui.Cursor = true
	gui.Mouse = false
	gui.InputEsc = true

	gui.Update(func(g *gocui.Gui) error {
		controlsView, err := g.View(controlsViewName)

		if err != nil {
			panic(err)
		}

		fmt.Fprint(controlsView, "\x1b[30;46m PgUp/PgDn: Scroll users \x1b[0m \x1b[30;46m F1: Help \x1b[0m \x1b[30;46m F10: Quit \x1b[0m")

		return nil
	})

	updateStatus()

	gui.MainLoop()
}
