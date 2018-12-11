package main

import (
	"fmt"
	"github.com/jroimartin/gocui"
	"strings"
	"time"
	"bytes"
)

const (
	logViewName       = "log"
	statusViewName    = "status"
	chatViewName      = "chat"
	usersViewName     = "users"
	chatInputViewName = "chatInput"
	controlsViewName  = "controls"

	logViewTitle    = "Log"
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

	if len(input) > 0 { 
		if input[0] == '/' {
			args := strings.Split(input, " ")
			processCommand(args[0], args[1:])
		} else {
			chatMessage(input)
		}
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
	case key == gocui.KeyF5:
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

func updateUsers(us []string) {
	var b bytes.Buffer

	for _, u := range(us) {
		b.WriteString(fmt.Sprintf("%s\n", u))
	}

	overwriteView(usersViewName, b.String())
}

func updateStatus() {
	if !debugEnabled {
		return
	}

	go func() {
		debugLog("updateStatus")

		time.Sleep(100 * time.Millisecond)

		networkGlobalsMutex.Lock()
		var nodesStr string
		if nodes != nil {
			nodes.lock.Lock()
			cn := nodes.head
			for cn != nil {
				cn.data.lock.Lock()
				nID := cn.data.id
				nodesStr = fmt.Sprintf("%s\n    -> \x1b[32m0x%X\x1b[0m (listening on %s): \x1b[33m%s\x1b[0m", nodesStr,
					nID, idToEndpoint(nID), cn.data.r)
				cn.data.lock.Unlock()
				cn = cn.next
			}
			nodes.lock.Unlock()
		}

		overwriteView(statusViewName, fmt.Sprintf(""+
			"  Logical time: \x1b[33;1m%d\x1b[0m\n"+
			" Network state: \x1b[33;1m%s\x1b[0m\n"+
			"            Node ID: \x1b[33;1m0x%X\x1b[0m (%s)\n"+
			" Twice Next Node ID: \x1b[33;1m0x%X\x1b[0m (%s)\n"+
			"          Leader ID: \x1b[33;1m0x%X\x1b[0m (%s)\n"+
			"\n"+
			" Connected nodes:\n%s\n\n   ----- END -----", readTime(),
			 readNetworkState(), nodeId, idToEndpoint(nodeId), getTwiceNextNodeId(),
			  idToEndpoint(getTwiceNextNodeId()), readLeaderId(), 
			  idToEndpoint(readLeaderId()), nodesStr))

		networkGlobalsMutex.Unlock()
	}()
}

func appendLogView(s string) {
	if !debugEnabled {
		return
	}

	appendView(logViewName, s+"\n")
}

func setConnectedName(n string) {
	gui.Update(func(g *gocui.Gui) error {
		v, _ := gui.View(chatInputViewName)
		if len(n) == 0 {
			v.Title = ""
		} else {
			v.Title = fmt.Sprintf(" Chatting as: %s ", n)
		}
		return nil
	})
}

func resetConnectedName() {
	setConnectedName("")
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

	controlsView.Wrap = true
	controlsView.Frame = false

	g.SetCurrentView(chatInputViewName)

	return nil
}

func scrollView(vn string, dy int) {
	// thanks to https://github.com/jroimartin/gocui/issues/84
    v, _ := gui.View(vn)
    
    _, y := v.Size()
    ox, oy := v.Origin()

    if oy+dy > strings.Count(v.ViewBuffer(), "\n")-y-1 {
        v.Autoscroll = true
    } else {
        v.Autoscroll = false
        v.SetOrigin(ox, oy+dy)
    }
}

func setupKeyBindings(g *gocui.Gui) {
	g.SetKeybinding("", gocui.KeyPgup, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		scrollView(usersViewName, -1)
		return nil
	})

	g.SetKeybinding("", gocui.KeyPgdn, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		scrollView(usersViewName, 1)
		return nil
	})

	g.SetKeybinding("", gocui.KeyF6, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		scrollView(chatViewName, -1)
		return nil
	})

	g.SetKeybinding("", gocui.KeyF7, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		scrollView(chatViewName, 1)
		return nil
	})

	g.SetKeybinding("", gocui.KeyF1, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		processCommand("/help", nil)
		return nil
	})

	g.SetKeybinding("", gocui.KeyF5, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		updateStatus()
		return nil
	})

	g.SetKeybinding("", gocui.KeyF10, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return gocui.ErrQuit
	})

	if debugEnabled {
		g.SetKeybinding("", gocui.KeyF11, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			scrollView(statusViewName, -1)
			return nil
		})

		g.SetKeybinding("", gocui.KeyF12, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			scrollView(statusViewName, 1)
			return nil
		})

		g.SetKeybinding("", gocui.KeyF8, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			scrollView(logViewName, -1)
			return nil
		})

		g.SetKeybinding("", gocui.KeyF9, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			scrollView(logViewName, 1)
			return nil
		})
	}
}

func initializeTui() {
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

	if debugEnabled {
		overwriteView(controlsViewName, "" +
			"\x1b[30;46m F1: Help \x1b[0m " +
			"\x1b[30;46m F5: Refresh status \x1b[0m " +
			"\x1b[30;46m F6/F7: Scroll chat \x1b[0m " +
			"\x1b[30;46m F8/F9: Scroll log \x1b[0m " +
			"\x1b[30;46m F10: Quit \x1b[0m " +
			"\x1b[30;46m F11/F12: Scroll status \x1b[0m " +
			"\x1b[30;46m PgUp/PgDn: Scroll users \x1b[0m ")
	} else {
		overwriteView(controlsViewName, "" +
			"\x1b[30;46m F1: Help \x1b[0m " +
			"\x1b[30;46m F5: Refresh status \x1b[0m " +
			"\x1b[30;46m F6/F7: Scroll chat \x1b[0m " +
			"\x1b[30;46m F10: Quit \x1b[0m " +
			"\x1b[30;46m PgUp/PgDn: Scroll users \x1b[0m ")
	}

	// hack to enable initial autoscroll on log and chat 
	gui.Update(func(g *gocui.Gui) error {
		if debugEnabled {
			scrollView(logViewName, 1)
		}
		scrollView(chatViewName, 1)
		return nil
	})

	userEvent(fmt.Sprintf("using default nickname \"%s\"", getChatName()))

	updateStatus()

	gui.MainLoop()
}
