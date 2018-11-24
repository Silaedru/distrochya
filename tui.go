package main

import (
	"fmt"
	"github.com/jroimartin/gocui"
)

const (
	LOG_VIEW_NAME = "log"
	STATUS_VIEW_NAME = "status"
	CHAT_VIEW_NAME = "chat"
	USERS_VIEW_NAME = "users"
	CHATINPUT_VIEW_NAME = "chatInput"
	CONTROLS_VIEW_NAME = "controls"
)

type chatInput struct {
}

var gui *gocui.Gui

func (e *chatInput) onEnter(v *gocui.View) {
	input := v.Buffer()

	clearView(CHATINPUT_VIEW_NAME)
	v.SetCursor(0, 0)

	AppendChat(input)
}

func (e *chatInput) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
		case key == gocui.KeyEnter: e.onEnter(v)
		case key == gocui.KeySpace: v.EditWrite(' ')
		
		case key == gocui.KeyBackspace || key == gocui.KeyBackspace2: v.EditDelete(true)
		case key == gocui.KeyDelete: v.EditDelete(false)
		case key == gocui.KeyArrowLeft: v.MoveCursor(-1, 0, false)
		case key == gocui.KeyArrowRight: v.MoveCursor(1, 0, false) 
		
		case key == gocui.KeyHome: v.SetCursor(0, 0)
		case key == gocui.KeyArrowUp: v.SetCursor(0, 0)

		case key == gocui.KeyArrowDown: v.SetCursor(len(v.Buffer())-1, 0)
		case key == gocui.KeyEnd: v.SetCursor(len(v.Buffer())-1, 0)

		case key == gocui.KeyF1: return
		case key == gocui.KeyF10: return

		case ch != 0 && mod == 0: v.EditWrite(ch)
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

func UpdateUsers() {

}

func UpdateStatus() {
	if !DebugEnabled {
		return
	}
}

func AppendLog(s string) {
	if !DebugEnabled {
		return
	}

	appendView(LOG_VIEW_NAME, s)
}

func AppendChat(s string) {
	appendView(CHAT_VIEW_NAME, s)
}

func layout(g *gocui.Gui) error {
	maxW, maxH := g.Size()

	maxW -= 1
	maxH -= 1

	curY := 0

	if DebugEnabled {
		logViewWidth := maxW * 2 / 3
		logViewHeight := maxH / 2

		logView, err := g.SetView(LOG_VIEW_NAME, 0, curY, logViewWidth, logViewHeight)

		if err != nil && err != gocui.ErrUnknownView {
			panic(err)
		}

		logView.Wrap = true
		logView.Title = LOG_VIEW_TITLE

		statusView, err := g.SetView(STATUS_VIEW_NAME, logViewWidth + 1, curY, maxW, logViewHeight)
		
		if err != nil && err != gocui.ErrUnknownView {
			panic(err)
		}

		statusView.Wrap = true
		statusView.Title = STATUS_VIEW_TITLE

		curY = logViewHeight+1
	}

	chatViewWidth := maxW * 4 / 5

	chatView, err := g.SetView(CHAT_VIEW_NAME, 0, curY, chatViewWidth, maxH-3)
	
	if err != nil && err != gocui.ErrUnknownView {
		panic(err)
	}

	chatView.Wrap = true
	chatView.Autoscroll = true
	chatView.Title = CHAT_VIEW_TITLE

	usersView, err := g.SetView(USERS_VIEW_NAME, chatViewWidth+1, curY, maxW, maxH - 2)
	
	if err != nil && err != gocui.ErrUnknownView {
		panic(err)
	}

	usersView.Wrap = false
	usersView.Title = USERS_VIEW_TITLE

	chatInputView, err := g.SetView(CHATINPUT_VIEW_NAME, 0, maxH - 4, chatViewWidth, maxH-2)

	if err != nil && err != gocui.ErrUnknownView {
		panic(err)
	}

	chatInputView.Editable = true
	chatInputView.Wrap = false
	chatInputView.Editor = &chatInput{}

	controlsView, err := g.SetView(CONTROLS_VIEW_NAME, 0, maxH - 2, maxW, maxH)
	
	if err != nil && err != gocui.ErrUnknownView {
		panic(err)
	}
	
	controlsView.Wrap = false
	controlsView.Frame = false

	g.SetCurrentView(CHATINPUT_VIEW_NAME)

	return nil
}

func setupKeyBindings(g *gocui.Gui) {
	g.SetKeybinding("", gocui.KeyPgup, gocui.ModNone, func (g *gocui.Gui, v *gocui.View) error { 
		clearView(CHAT_VIEW_NAME)
		return nil
	})

	g.SetKeybinding("", gocui.KeyPgdn, gocui.ModNone, func (g *gocui.Gui, v *gocui.View) error { 
		return nil
	})

	g.SetKeybinding("", gocui.KeyF1, gocui.ModNone, func (g *gocui.Gui, v *gocui.View) error { 
		return nil
	})

	g.SetKeybinding("", gocui.KeyF10, gocui.ModNone, func (g *gocui.Gui, v *gocui.View) error { 
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
		controlsView, err := g.View(CONTROLS_VIEW_NAME)

		if err != nil {
			panic(err)
		}

		fmt.Fprint(controlsView, "\x1b[30;46m PgUp: Scroll users up \x1b[0m \x1b[30;46m PgDn: Scroll users down \x1b[0m \x1b[30;46m F1: Commands \x1b[0m \x1b[30;46m F10: Quit \x1b[0m")

		return nil
	})

	gui.MainLoop()
}