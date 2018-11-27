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

		go StartNetwork(uint16(port))
	}}

	commands["/disconnect"] = &command{"Disconnects from a network.", "", func (args[] string) {
		Disconnect()
	}}

	commands["/connect"] = &command{"Connects to an existing network", "<dest> <server port>", func (args[] string) {

	}}
}

func main() {
	initCommands()
	InitializeTui()
}