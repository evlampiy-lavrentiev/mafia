package server

// MafiaPlayer interface describes possible actions of the mafia game session player
type MafiaPlayer interface {
	SetName(string)
	GetName() string
	SetRole(string)
	GetRole() string
	IsActive() bool
	SetActive(bool)
	// TODO: replace with pointer
	Notify(Notification)
	GetNotification() (Notification, error)
	CancelNotifications()
	Vote(string)
	WaitForVote() string
	EndDay()
	WaitEndDay() (string, bool)
	SetExposed(string)
	Expose() (string, error)
}

type mafiaPlayer struct {
	name                string
	role                string
	active              bool
	notificationChannel chan Notification
	voteChannel         chan string
	endDayChannel       chan int
	exposedPlayer       string
}

func (p *mafiaPlayer) SetName(newName string) {
	p.name = newName
}

func (p *mafiaPlayer) GetName() string {
	return p.name
}

func (p *mafiaPlayer) SetRole(role string) {
	p.role = role
}

func (p *mafiaPlayer) GetRole() string {
	return p.role
}

func (p *mafiaPlayer) SetExposed(player string) {
	p.exposedPlayer = player
}

func (p *mafiaPlayer) Expose() (string, error) {
	if p.exposedPlayer != "" {
		return p.exposedPlayer, nil
	}

	return "", noExposedPlayerError
}

func (p *mafiaPlayer) SetActive(status bool) {
	p.active = status
}

func (p *mafiaPlayer) IsActive() bool {
	return p.active
}

func (p *mafiaPlayer) Notify(msg Notification) {
	p.notificationChannel <- msg
}

// TODO: maybe replace with pointer to Notification
func (p *mafiaPlayer) GetNotification() (Notification, error) {
	event, ok := <-p.notificationChannel
	if ok {
		return event, nil
	}

	return event, channelClosedError
}

func (p *mafiaPlayer) CancelNotifications() {
	close(p.notificationChannel)
	close(p.voteChannel)
	close(p.endDayChannel)
}

func (p *mafiaPlayer) Vote(target string) {
	p.voteChannel <- target
}

func (p *mafiaPlayer) WaitForVote() string {
	return <-p.voteChannel
}

func (p *mafiaPlayer) EndDay() {
	p.endDayChannel <- 1
}

func (p *mafiaPlayer) WaitEndDay() (string, bool) {
	select {
	case voteRes := <-p.voteChannel:
		//fmt.Printf("DEBUG <<<< GOT vote chan %s\n", voteRes)
		return voteRes, false
	case <-p.endDayChannel:
		//fmt.Printf("DEBUG <<<< GOT end chan\n")
		return "", true
	}
}
