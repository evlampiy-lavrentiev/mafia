package server

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

type MafiaSession interface {
	Start()
	PlayerVote(id uint64, target string)
	PlayerEndDay(id uint64)
	PlayerExpose(id uint64)
	AddPlayer(id uint64, name string) error
	RemovePlayer(id uint64)
	GetPlayersRole(id uint64) string
	SetPlayersRole(id uint64, role string)
	GetPlayersName(id uint64) string
	SetPlayersName(id uint64, name string)
	GetPlayersCount() int
	GetConnectedPlayers() []string
	HasStarted() bool
	NotifyPlayers(msg Notification, role string)
	SendChatMsg(id uint64, msg string)
	GetPlayersNotifications(id uint64) (Notification, error)
	UnsubscribePlayerFromNotifications(id uint64)
}

type mafiaSession struct {
	players              map[uint64]MafiaPlayer
	inProcess            bool
	phase                int
	mafiaAlive           int
	civilianAlive        int
	potentialVictims     map[string]int
	roundCnt             int
	delayedNotifications []Notification
	lock                 sync.Mutex
	waitGr               sync.WaitGroup
}

func (ms *mafiaSession) SendChatMsg(id uint64, msg string) {
	if ms.players[id].GetRole() == GHOST {
		ms.players[id].Notify(Notification{CHAT_RESTRICTED, "Ghosts are not allowed to send any messages"})
		return
	}

	if ms.phase == DAY {
		ms.NotifyPlayers(Notification{CHAT_MSG, ms.players[id].GetName() + "@@" + msg}, ALL)
	} else {
		if ms.players[id].GetRole() != MAFIA {
			ms.players[id].Notify(Notification{CHAT_RESTRICTED, "only mafia can communicate at night"})
			return
		}
		ms.NotifyPlayers(Notification{CHAT_MSG, ms.players[id].GetName() + "@@" + msg}, MAFIA)
	}
}

func (ms *mafiaSession) snapshot() {
	fmt.Println(
		"-- SNAPSHOT --\n",
		fmt.Sprintf("Players: %v\n", ms.players),
		fmt.Sprintf("InProcess: %v\n", ms.inProcess),
		fmt.Sprintf("Phase: %d\n", ms.phase),
		fmt.Sprintf("mafiaAlive: %d\n", ms.mafiaAlive),
		fmt.Sprintf("civilianAlive: %d\n", ms.civilianAlive),
		fmt.Sprintf("potentialVictims: %v\n", ms.potentialVictims),
		fmt.Sprintf("roundCnt: %d\n", ms.roundCnt),
		fmt.Sprintf("delayedNotifications: %v\n", ms.delayedNotifications),
	)
}

func (ms *mafiaSession) debug(msg string) {
	fmt.Println("DEBUG <<<<", msg)
}

func (ms *mafiaSession) nameTaken(name string) bool {
	ms.debug(fmt.Sprintf("Testing name %s", name))
	for _, player := range ms.players {
		if player.GetName() == name {
			return true
		}
	}

	ms.debug("No such name")
	return false
}

func (ms *mafiaSession) AddPlayer(id uint64, name string) error {
	if !ms.nameTaken(name) {
		ms.players[id] = &mafiaPlayer{
			name:                name,
			active:              false,
			notificationChannel: make(chan Notification, 10),
			voteChannel:         make(chan string, 1),
			endDayChannel:       make(chan int, 1),
			exposedPlayer:       "",
		}
		return nil
	}

	return nameCollisionError
}

func (ms *mafiaSession) RemovePlayer(id uint64) {
	if ms.players[id].GetRole() == MAFIA {
		ms.mafiaAlive--
	} else if ms.players[id].GetRole() == CIVILIAN {
		ms.civilianAlive--
	}

	if ms.inProcess && ms.endGameConditionReached() {
		ms.waitGr.Done()
		ms.end()
	}

	delete(ms.players, id)
}

func (ms *mafiaSession) getPlayersIdByName(name string) (uint64, error) {
	for id, player := range ms.players {
		if player.GetName() == name {
			return id, nil
		}
	}

	return 0, playerRemovedError
}

func (ms *mafiaSession) GetPlayersRole(id uint64) string {
	return ms.players[id].GetRole()
}

func (ms *mafiaSession) SetPlayersRole(id uint64, role string) {
	ms.players[id].SetRole(role)
}

func (ms *mafiaSession) GetPlayersName(id uint64) string {
	//ms.snapshot()
	return ms.players[id].GetName()
}

func (ms *mafiaSession) SetPlayersName(id uint64, name string) {
	ms.players[id].SetName(name)
}

func (ms *mafiaSession) passVoteConditions(id uint64) bool {
	if !ms.players[id].IsActive() {
		ms.players[id].Notify(Notification{VOTING_RESTRICTED, "you have already voted"})
	} else if ms.roundCnt == 0 && ms.phase == DAY {
		ms.players[id].Notify(Notification{VOTING_RESTRICTED, "you are not allowed to vote on the first day"})
	} else if ms.players[id].GetRole() == GHOST {
		ms.players[id].Notify(Notification{VOTING_RESTRICTED, "you may only spectate as a ghost"})
	} else if ms.phase == NIGHT && ms.players[id].GetRole() != MAFIA && ms.players[id].GetRole() != DETECTIVE {
		ms.players[id].Notify(Notification{VOTING_RESTRICTED, "only mafia members and detectives are allowed to vote at night"})
	} else {
		return true
	}

	return false
}

func (ms *mafiaSession) PlayerVote(id uint64, target string) {
	ms.debug("VOTE")
	if ms.passVoteConditions(id) {
		ms.players[id].Vote(target)
	}
}

func (ms *mafiaSession) passEndDayConditions(id uint64) bool {
	if !ms.players[id].IsActive() {
		ms.players[id].Notify(Notification{VOTING_RESTRICTED, "you have already skipped the current day"})
	} else if ms.players[id].GetRole() == GHOST {
		ms.players[id].Notify(Notification{VOTING_RESTRICTED, "you may only spectate as a ghost"})
	} else if ms.phase == NIGHT {
		ms.players[id].Notify(Notification{VOTING_RESTRICTED, "you can't end day during night phase"})
	} else {
		return true
	}

	return false
}

func (ms *mafiaSession) PlayerEndDay(id uint64) {
	ms.debug("EndDay")
	if ms.passEndDayConditions(id) {
		ms.players[id].EndDay()
		ms.players[id].SetActive(false)
	}
}

func (ms *mafiaSession) passExposeConditions(id uint64) bool {
	if !ms.players[id].IsActive() {
		ms.players[id].Notify(Notification{VOTING_RESTRICTED, "you have already voted"})
	} else if ms.players[id].GetRole() != DETECTIVE {
		ms.players[id].Notify(Notification{VOTING_RESTRICTED, "only detective can expose players"})
	} else if ms.phase != DAY {
		ms.players[id].Notify(Notification{VOTING_RESTRICTED, "you may expose players only at night"})
	} else {
		return true
	}

	return false
}

func (ms *mafiaSession) PlayerExpose(id uint64) {
	ms.debug("Expose")
	if ms.passExposeConditions(id) {
		exposedName, err := ms.players[id].Expose()
		if err != nil {
			log.Println(err.Error())
			ms.players[id].Notify(Notification{eventType: NO_EXPOSED_PLAYER})
		} else {
			ms.NotifyPlayers(Notification{PLAYER_EXPOSED, exposedName}, ALL)
		}
	}
}

func (ms *mafiaSession) GetPlayersCount() int {
	return len(ms.players)
}

func (ms *mafiaSession) GetConnectedPlayers() []string {
	res := make([]string, len(ms.players))
	for _, player := range ms.players {
		res = append(res, player.GetName())
	}

	return res
}

func (ms *mafiaSession) GetPlayersNotifications(id uint64) (Notification, error) {
	player, ok := ms.players[id]
	if ok {
		return player.GetNotification()
	}

	return Notification{}, playerRemovedError
}

func (ms *mafiaSession) UnsubscribePlayerFromNotifications(id uint64) {
	ms.players[id].CancelNotifications()
}

func (ms *mafiaSession) NotifyPlayers(msg Notification, scope string) {
	for _, player := range ms.players {
		if scope == ALL || scope == player.GetRole() {
			player.Notify(msg)
		}
	}
}

func (ms *mafiaSession) deliverDelayedNotifications() {
	for _, notification := range ms.delayedNotifications {
		ms.NotifyPlayers(notification, ALL)
	}
	ms.delayedNotifications = ms.delayedNotifications[:0]
}

func (ms *mafiaSession) HasStarted() bool {
	return ms.inProcess
}

func (ms *mafiaSession) shuffleRoles() {
	var (
		playerCnt              = len(ms.players)
		mafiaCnt, detectiveCnt = calcRoleQuota(playerCnt, MAFIA), calcRoleQuota(playerCnt, DETECTIVE)
	)

	shuffleOrder := rand.Perm(playerCnt)
	var (
		currentInd       = 0
		nextMafiaInd     = 0
		nextDetectiveInd = mafiaCnt
		nextCivillianInd = mafiaCnt + detectiveCnt
		curRole          = ""
	)

	for _, player := range ms.players {
		fmt.Println("INDEXES ARE AS FOLLOWING:\n",
			fmt.Sprintf("\tcur ind %d\n", currentInd),
			fmt.Sprintf("\tmafia %d %d\n", nextMafiaInd, mafiaCnt),
			fmt.Sprintf("\tdetective %d %d\n", nextDetectiveInd, detectiveCnt),
			fmt.Sprintf("\tcivillian %d %d\n", nextCivillianInd, playerCnt-mafiaCnt-detectiveCnt),
			fmt.Sprintf("\tperm %v\n", shuffleOrder),
		)
		switch currentInd {
		case shuffleOrder[nextMafiaInd]:
			curRole = MAFIA
			player.SetRole(MAFIA)
			if nextMafiaInd < mafiaCnt-1 {
				nextMafiaInd++
			}
		case shuffleOrder[nextDetectiveInd]:
			curRole = DETECTIVE
			player.SetRole(DETECTIVE)
			if nextDetectiveInd < detectiveCnt+mafiaCnt-1 {
				nextDetectiveInd++
			}
		default:
			curRole = CIVILIAN
			player.SetRole(CIVILIAN)
			nextCivillianInd++
		}
		currentInd++
		player.Notify(Notification{ROLE_ASSIGNED, curRole})
	}

	ms.mafiaAlive = mafiaCnt
	ms.civilianAlive = playerCnt - mafiaCnt - detectiveCnt
}

func (ms *mafiaSession) endGameConditionReached() bool {
	return ms.mafiaAlive == 0 || ms.mafiaAlive == ms.civilianAlive
}

func (ms *mafiaSession) carryOutExecution() {
	ms.debug("carryOutExecution")
	//ms.snapshot()
	if ms.phase == DAY {
		var (
			maxVotes   = 0
			collisions = 0
			target     = ""
		)

		ms.debug("VICTIMS")
		for victim, votes := range ms.potentialVictims {
			fmt.Println(victim+": ", votes)
			if votes > maxVotes {
				maxVotes = votes
				collisions = 0
				target = victim
			} else if votes == maxVotes {
				collisions++
			}
		}

		fmt.Println("Collisions", collisions)
		fmt.Println("target", target)
		if collisions == 0 && target != "" {
			confirmedVictimId, err := ms.getPlayersIdByName(target)
			if err != nil {
				ms.debug("DAY VICTIM ERROR")
				//ms.snapshot()
				log.Println(err.Error())
				ms.NotifyPlayers(Notification{PLAYER_NOT_FOUND, target}, ALL)
				ms.potentialVictims = make(map[string]int)
				return
			}
			if ms.players[confirmedVictimId].GetRole() == MAFIA {
				ms.mafiaAlive--
			} else if ms.players[confirmedVictimId].GetRole() == CIVILIAN {
				ms.civilianAlive--
			}
			ms.NotifyPlayers(Notification{PLAYER_ELIMINATED, ms.players[confirmedVictimId].GetName() + " " + ms.players[confirmedVictimId].GetRole()}, ALL)
			ms.players[confirmedVictimId].SetRole(GHOST)
		} else {
			ms.NotifyPlayers(Notification{eventType: VOTES_MISMATCH}, ALL)
		}
		ms.potentialVictims = make(map[string]int)
	} else {
		if len(ms.potentialVictims) != 1 {
			ms.NotifyPlayers(Notification{eventType: MAFIA_VOTES_MISMATCH}, MAFIA)
		}

		for victim := range ms.potentialVictims {
			confirmedVictimId, err := ms.getPlayersIdByName(victim)
			if err != nil {
				ms.debug("NIGHT VICTIM ERROR")
				//ms.snapshot()
				log.Println(err.Error())
				ms.NotifyPlayers(Notification{PLAYER_NOT_FOUND, victim}, MAFIA)
				break
			}
			if ms.players[confirmedVictimId].GetRole() == MAFIA {
				ms.mafiaAlive--
			} else if ms.players[confirmedVictimId].GetRole() == CIVILIAN {
				ms.civilianAlive--
			}
			// Notification will be shown only at the beginning of the Next Day
			ms.delayedNotifications = append(ms.delayedNotifications, Notification{PLAYER_ELIMINATED, ms.players[confirmedVictimId].GetName() + " " + ms.players[confirmedVictimId].GetRole()})
			ms.players[confirmedVictimId].SetRole(GHOST)
		}
		ms.potentialVictims = make(map[string]int)
	}
	ms.debug("SUCCESS")
	//ms.snapshot()

}

func (ms *mafiaSession) runRound() {
	ms.debug("runRound")
	//ms.snapshot()

	for _, player := range ms.players {
		player.SetActive(true)
	}

	if ms.phase == DAY {
		ms.NotifyPlayers(Notification{eventType: PHASE_START_DAY}, ALL)

		time.Sleep(NOTIFICATION_DELAY)
		ms.deliverDelayedNotifications()

		ms.debug("WAIT ON SKIP")
		for _, player := range ms.players {
			if player.GetRole() == GHOST {
				continue
			}
			ms.waitGr.Add(1)

			// only the last vote will be counted
			go func(player MafiaPlayer, mutex *sync.Mutex, wGroup *sync.WaitGroup) {
				var lastVote string

				for voteRes, dayEnded := player.WaitEndDay(); !dayEnded; voteRes, dayEnded = player.WaitEndDay() {
					if voteRes != "" {
						lastVote = voteRes
					}
				}

				mutex.Lock()
				if _, isAlreadyAVictim := ms.potentialVictims[lastVote]; isAlreadyAVictim {
					ms.potentialVictims[lastVote] += 1
				} else {
					ms.potentialVictims[lastVote] = 0
				}
				mutex.Unlock()
				wGroup.Done()
			}(player, &ms.lock, &ms.waitGr)
		}
		ms.waitGr.Wait()
		ms.carryOutExecution()
		ms.phase = NIGHT
	} else {
		ms.NotifyPlayers(Notification{eventType: PHASE_START_NIGHT}, ALL)
		ms.debug("WAITING ON NIGHT VOTES")
		for _, player := range ms.players {
			if player.GetRole() != MAFIA && player.GetRole() != DETECTIVE {
				ms.debug(fmt.Sprintf("Player %s skipped", player.GetName()))
				continue
			}

			ms.waitGr.Add(1)
			go func(player MafiaPlayer, mutex *sync.Mutex, wGroup *sync.WaitGroup) {
				voteRes := player.WaitForVote()
				for ; !ms.nameTaken(voteRes); voteRes = player.WaitForVote() {
					player.Notify(Notification{PLAYER_NOT_FOUND, voteRes})
				}
				ms.debug(fmt.Sprintf("Player %s voted for %s", player.GetName(), voteRes))
				player.SetActive(false)
				if player.GetRole() == DETECTIVE {
					suspectId, err := ms.getPlayersIdByName(voteRes)
					if err != nil {
						log.Println(err.Error())
						player.Notify(Notification{PLAYER_NOT_FOUND, voteRes})
						wGroup.Done()
						return
					}

					if suspect := ms.players[suspectId]; suspect.GetRole() == MAFIA {
						player.SetExposed(suspect.GetName())
						player.Notify(Notification{eventType: GUESS_SUCCESS})
					} else {
						player.SetExposed("")
						player.Notify(Notification{eventType: GUESS_FAIL})
					}
				} else {
					mutex.Lock()
					if _, isAlreadyAVictim := ms.potentialVictims[voteRes]; isAlreadyAVictim {
						ms.potentialVictims[voteRes] += 1
					} else {
						ms.potentialVictims[voteRes] = 0
					}
					mutex.Unlock()
				}
				wGroup.Done()
			}(player, &ms.lock, &ms.waitGr)
		}
		ms.waitGr.Wait()
		ms.carryOutExecution()
		ms.roundCnt++
		ms.phase = DAY
	}
}

func (ms *mafiaSession) Start() {
	if len(ms.players) < PLAYERS_LOWER_LIM {
		ms.NotifyPlayers(Notification{eventType: SESSION_ABORT}, ALL)
		return
	}

	ms.inProcess = true
	ms.roundCnt = 0
	ms.phase = DAY
	ms.shuffleRoles()
	log.Println("GAME SESSION STARTED")
	ms.NotifyPlayers(Notification{eventType: SESSION_START}, ALL)
	for !ms.endGameConditionReached() {
		ms.runRound()
	}
	//ms.snapshot()
	ms.end()
}

func (ms *mafiaSession) end() {
	ms.inProcess = false
	log.Println("GAME SESSION ENDED")
	if ms.mafiaAlive == 0 {
		ms.NotifyPlayers(Notification{SESSION_END, "civilians have won"}, ALL)
	} else {
		ms.NotifyPlayers(Notification{SESSION_END, "mafia has won"}, ALL)
	}

	//for _, player := range ms.players {
	//	player.CancelNotifications()
	//}
	//
	//ms.players = make(map[uint64]MafiaPlayer)
	ms.potentialVictims = make(map[string]int)
	ms.delayedNotifications = ms.delayedNotifications[:0]
}
