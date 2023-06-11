package client

import (
	"fmt"
	"strings"
)

type command uint16

const (
	HELP command = iota
	EXIT
	CONNECT
	DISCONNECT
	SHOW_PLAYER_LIST
	VOTE
	END_DAY
	EXPOSE
	CHAT
	UNKNOWN
)

func showHints() {
	fmt.Println("",
		"'connect':\t join a game server\n",
		"'exit':\t exit client\n",
		"'players':\t show players in the game session\n",
		"'vote':\t vote for a player\n",
		"'expose':\t expose mafia if you are a detective\n",
		"'skip':\t end your turn in the current day\n",
		"'chat':\t send a message in chat",
	)
}

func (c command) toString() string {
	switch c {
	case CONNECT:
		return "connect"
	case DISCONNECT:
		return "disconnect"
	case SHOW_PLAYER_LIST:
		return "players"
	case VOTE:
		return "vote"
	case EXPOSE:
		return "expose"
	case END_DAY:
		return "skip"
	case EXIT:
		return "exit"
	case CHAT:
		return "chat"
	case HELP:
		return "help"
	default:
		return "undefined"
	}
}

func parseCommand(cmd string) command {
	switch strings.ToLower(cmd) {
	case CONNECT.toString():
		return CONNECT
	case DISCONNECT.toString():
		return DISCONNECT
	case SHOW_PLAYER_LIST.toString():
		return SHOW_PLAYER_LIST
	case VOTE.toString():
		return VOTE
	case EXPOSE.toString():
		return EXPOSE
	case END_DAY.toString():
		return END_DAY
	case EXIT.toString():
		return EXIT
	case CHAT.toString():
		return CHAT
	case HELP.toString():
		return HELP
	default:
		return UNKNOWN
	}
}
