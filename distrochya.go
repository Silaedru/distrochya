package main

import (
	"fmt"
	"strconv"
)

type command struct {
	helpString string
	usage      string
	callback   func([]string)
}

var debugEnabled bool = true
var debugLogEnabled bool = false
var commands map[string]*command

func processCommand(name string, args []string) {
	command := commands[name]

	if command == nil {
		userError("unknown command " + name)
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
	appendLogView(fmt.Sprintf("(%8d)  %s", advanceTime(), m))
}

func userError(e string) {
	appendChatView(fmt.Sprintf("Error: %s", e))
}

func userEvent(m string) {
	appendChatView(fmt.Sprintf("Info: %s", m))
}

func initCommands() {
	commands = make(map[string]*command)

	commands["/help"] = &command{"Prints this message.", "", func(args []string) {
		msg := "\nAvailable commands:"

		for n, c := range commands {
			msg = fmt.Sprintf("%s\n%s %s   \t%s", msg, n, c.usage, c.helpString)
		}

		appendChatView(msg + "\n")
	}}

	commands["/start"] = &command{"Starts a new network. Node will listen for incoming connections on specified <port>.", "<port>", func(args []string) {
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

	commands["/disconnect"] = &command{"Disconnects from a network.", "", func(args []string) {
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

	commands["/us"] = &command{"Update status", "", func(args []string) {
		updateStatus()
	}}

	commands["/a"] = &command{"/start 9999", "", func(args []string) {
		processCommand("/start", []string{"9999"})
	}}
	commands["/b"] = &command{"/connect localhost:9999 9998", "", func(args []string) {
		processCommand("/connect", []string{"localhost:9999", "9998"})
	}}
	commands["/c"] = &command{"/connect localhost:9999 9997", "", func(args []string) {
		processCommand("/connect", []string{"localhost:9999", "9997"})
	}}
	commands["/d"] = &command{"/connect localhost:9999 9996", "", func(args []string) {
		processCommand("/connect", []string{"localhost:9999", "9996"})
	}}

	commands["/m"] = &command{"mark", "", func(args []string) {
		appendChatView("========= MARK ==========")
		appendLogView("========= MARK ==========")
	}}
}

func main() {
	initCommands()
	initializeTui()
}
