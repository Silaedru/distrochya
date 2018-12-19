package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

type command struct {
	helpString string
	usage      string
	callback   func([]string)
}

const debugLogEnabled = false

var debugEnabled = true
var commands map[string]*command

func processCommand(name string, args []string) {
	command := commands[name]

	if command == nil {
		userError(fmt.Sprintf("unknown command \"%s\"", name))
		return
	}

	command.callback(args)
}

func debugLog(m string) {
	if !debugLogEnabled {
		return
	}

	appendLogView("DEBUG: " + m)
}

func log(m string) {
	appendLogView(fmt.Sprintf("\x1b[37;1m(%8d)\x1b[0m  %s", advanceTime(), m))
}

func userError(e string) {
	appendChatView(fmt.Sprintf("<\x1b[31mError\x1b[0m>: %s", e))
}

func userEvent(m string) {
	appendChatView(fmt.Sprintf("<\x1b[35mInfo\x1b[0m>: %s", m))
}

func chatMessageReceived(u string, s string) {
	appendChatView(fmt.Sprintf("<\x1b[32m%s\x1b[0m>: %s", u, s))
}

func initCommands() {
	commands = make(map[string]*command)

	commands["/help"] = &command{"Prints this message.", "                       ", func(args []string) {
		msg := "\nAvailable commands:"

		for n, c := range commands {
			msg = fmt.Sprintf("%s\n%s %s          %s", msg, n, c.usage, c.helpString)
		}

		appendChatView(msg + "\n")
	}}

	commands["/start"] = &command{"Starts a new network. Node will listen for incoming connections on specified <port>.",
		"<port>                ", func(args []string) {
			if args == nil || len(args) != 1 {
				userError("invalid usage")
				return
			}

			port, err := strconv.ParseUint(args[0], 10, 16)

			if err != nil {
				userError("failed to parse port number")
				return
			}

			startNetwork(uint16(port))
		}}

	commands["/disconnect"] = &command{"Disconnects from a network.", "                 ", func(args []string) {
		disconnect()
	}}

	commands["/connect"] = &command{"Connects to an existing network", "<dest> <server port>", func(args []string) {
		if args == nil || len(args) != 2 {
			userError("invalid usage")
			return
		}

		port, err := strconv.ParseUint(args[1], 10, 16)

		if err != nil {
			userError("failed to parse port number")
			return
		}

		joinNetwork(args[0], uint16(port))
	}}

	commands["/nick"] = &command{"Sets a new nickname", "[new nickname]         ", func(args []string) {
		if len(args) > 0 {
			var nick bytes.Buffer

			for i, s := range args {
				nick.WriteString(s)

				if i+1 < len(args) {
					nick.WriteString(" ")
				}
			}

			nickStr := strings.Replace(nick.String(), ";", "", -1)
			setChatName(nickStr)
		}

		appendChatView(fmt.Sprintf("\x1b[35mNickname: %s\x1b[0m", getChatName()))
	}}

	commands["/clear"] = &command{"Clears chat", "                      ", func(args []string) {
		clearView(chatViewName)
	}}

	commands["/setpart"] = &command{"Sets chat participation", "[new value]         ", func(args []string) {
		if len(args) > 0 {
			if value, err := strconv.Atoi(args[0]); err == nil {
				if value > 0 {
					setChatParticipation()
				} else {
					resetChatParticipation()
				}
			}
		}

		appendChatView(fmt.Sprintf("\x1b[35mChat participation: %d\x1b[0m", getChatParticipation()))
	}}

	if debugEnabled {
		commands["/us"] = &command{"Update status", "                         ", func(args []string) {
			updateStatus()
		}}

		commands["/cl"] = &command{"Clears log", "                         ", func(args []string) {
			clearView(logViewName)
		}}

		commands["/a"] = &command{"/start 9999", "                          ", func(args []string) {
			processCommand("/start", []string{"9999"})
		}}
		commands["/b"] = &command{"/connect localhost:9999 9998", "                          ", func(args []string) {
			processCommand("/connect", []string{"localhost:9999", "9998"})
		}}
		commands["/c"] = &command{"/connect localhost:9999 9997", "                          ", func(args []string) {
			processCommand("/connect", []string{"localhost:9999", "9997"})
		}}
		commands["/d"] = &command{"/connect localhost:9999 9996", "                          ", func(args []string) {
			processCommand("/connect", []string{"localhost:9999", "9996"})
		}}
		commands["/e"] = &command{"/connect localhost:9999 9995", "                          ", func(args []string) {
			processCommand("/connect", []string{"localhost:9999", "9995"})
		}}

		commands["/m"] = &command{"mark", "                          ", func(args []string) {
			appendChatView("========= MARK ==========")
			appendLogView("========= MARK ==========")
		}}
	}
}

func main() {
	args := os.Args[1:]

	for _, arg := range args {
		if arg == "--nodebug" {
			debugEnabled = false
		}
	}

	rand.Seed(time.Now().UnixNano())
	initCommands()
	initTUI()
}
