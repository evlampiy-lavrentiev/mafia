package server

// ---- player roles
const (
	MAFIA     = "mafia"
	DETECTIVE = "detective"
	CIVILIAN  = "civilian"
	GHOST     = "ghost"
	ALL       = ""
)

// calcRoleQuota returns number of players for a certain role, based on number of players
func calcRoleQuota(numberOfPlayers int, role string) int {
	switch role {
	case MAFIA:
		if numberOfPlayers >= PLAYERS_LOWER_LIM && numberOfPlayers < PLAYERS_MID_LIM {
			return 1
		} else if numberOfPlayers >= PLAYERS_MID_LIM && numberOfPlayers < PLAYERS_UPPER_LIM {
			return 2
		} else if numberOfPlayers >= PLAYERS_UPPER_LIM {
			return numberOfPlayers / 4
		}
	case DETECTIVE:
		if numberOfPlayers >= PLAYERS_LOWER_LIM && numberOfPlayers < PLAYERS_UPPER_LIM {
			return 1
		} else if numberOfPlayers >= PLAYERS_UPPER_LIM {
			return numberOfPlayers / 8
		}
	case CIVILIAN:
		if numberOfPlayers >= PLAYERS_LOWER_LIM && numberOfPlayers < PLAYERS_MID_LIM {
			return numberOfPlayers - 2
		} else if numberOfPlayers >= PLAYERS_MID_LIM && numberOfPlayers < PLAYERS_UPPER_LIM {
			return numberOfPlayers - 3
		} else if numberOfPlayers >= PLAYERS_UPPER_LIM {
			return numberOfPlayers - numberOfPlayers/4 - numberOfPlayers/8
		}
	default:
		return -1
	}

	// < min number of players given
	return -1
}

// ---- phase shift
const (
	DAY = iota
	NIGHT
)
