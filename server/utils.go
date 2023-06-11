package server

import "errors"

// ---- notifications
type notificationEvent uint16

const (
	CLIENT_CONNECTED notificationEvent = iota
	CLIENT_DISCONNECTED
	SESSION_DISCLAIMER
	SESSION_START
	SESSION_ABORT
	SESSION_END
	ROLE_ASSIGNED
	PLAYER_NOT_FOUND
	PLAYER_ELIMINATED
	PLAYER_EXPOSED
	NO_EXPOSED_PLAYER
	GUESS_SUCCESS
	GUESS_FAIL
	VOTING_RESTRICTED
	VOTES_MISMATCH
	MAFIA_VOTES_MISMATCH
	PHASE_START_DAY
	PHASE_START_NIGHT
	CHAT_MSG
	CHAT_RESTRICTED
)

type Notification struct {
	eventType notificationEvent
	info      string
}

// ---- custom errors
var nameCollisionError = errors.New("there is already a player with the same name in the session")
var sessionStartedError = errors.New("game session has already started, try to connect later")
var channelClosedError = errors.New("this player's Notification channel has been closed")
var playerRemovedError = errors.New("this player has already left the session")
var noExposedPlayerError = errors.New("you haven't exposed anyone during last night")
