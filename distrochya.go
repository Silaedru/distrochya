package main

import (
	"fmt"
	"strconv"
)

type command struct {
	helpString string
	usage string
	callback func([]string)
}

var DebugEnabled bool = true
var DebugLogEnabled bool = true
var commands map[string]*command


func ProcessCommand(name string, args []string) {
	command := commands[name]

	if command == nil {
		UserError("unknown command " + name)
		return
	}

	command.callback(args)
}

func ChatMessage(m string) {

}

func DebugLog(m string) {
	if !DebugLogEnabled {
		return
	}

	AppendLog("DEBUG: " + m)
}

func Log(m string) {
	AppendLog(m)
}

func UserError(e string) {
	AppendChat(fmt.Sprintf("Error: %s", e))
}

func UserEvent(m string) {
	AppendChat(fmt.Sprintf("Info: %s", m))
}

func initCommands() {
	commands = make(map[string]*command)

	commands["/help"] = &command{"Prints this message.", "", func (args[] string) {
		msg := "\nAvailable commands:"

		for n, c := range commands {
			msg = fmt.Sprintf("%s\n%s %s   \t%s", msg, n, c.usage, c.helpString)
		}

		AppendChat(msg + "\n")
	}}

	commands["/start"] = &command{"Starts a new network. Node will listen for incoming connections on specified <port>.", "<port>", func (args[] string) {
		if args == nil || len(args) != 1 {
			UserError("invalid usage")
			return
		}

		port, err := strconv.ParseUint(args[0], 10, 16)

		if err != nil {
			UserError("failed to parse port number")
			return
		}

		StartNetwork(uint16(port))
	}}

	commands["/disconnect"] = &command{"Disconnects from a network.", "", func (args[] string) {
		Disconnect()
	}}

	commands["/connect"] = &command{"Connects to an existing network", "<dest> <server port>", func (args[] string) {
		if args == nil || len(args) != 2 {
			UserError("invalid usage")
			return
		}

		port, err := strconv.ParseUint(args[1], 10, 16)

		if err != nil {
			UserError("failed to parse port number")
			return
		}

		JoinNetwork(args[0], uint16(port))
	}}

	commands["/us"] = &command{"Update status", "", func (args[] string) {
		UpdateStatus()
	}}

	commands["/a"] = &command{"/start 9999", "", func (args[] string) {
		ProcessCommand("/start", []string{"9999"})
	}}
	commands["/b"] = &command{"/connect localhost:9999 9998", "", func (args[] string) {
		ProcessCommand("/connect", []string{"localhost:9999", "9998"})
	}}
	commands["/c"] = &command{"/connect localhost:9999 9997", "", func (args[] string) {
		ProcessCommand("/connect", []string{"localhost:9999", "9997"})
	}}
	commands["/d"] = &command{"/connect localhost:9999 9996", "", func (args[] string) {
		ProcessCommand("/connect", []string{"localhost:9999", "9996"})
	}}
}

func main() {
	initCommands()
	InitializeTui()
}