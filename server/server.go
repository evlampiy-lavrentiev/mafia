package server

import (
	"context"
	"fmt"
	"log"
	"mafia-core/proto"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
)

type server struct {
	proto.UnimplementedMafiaServer
	session      MafiaSession
	nextClientId uint64
	sessionStart chan int
	mutex        sync.Mutex
}

func (s *server) Connect(_ context.Context, req *proto.ClientInfo) (*proto.ClientId, error) {
	if s.session.HasStarted() {
		return &proto.ClientId{Id: 42}, sessionStartedError
	}

	s.mutex.Lock()
	clientId := s.nextClientId
	err := s.session.AddPlayer(clientId, req.Name)
	if err != nil {
		return &proto.ClientId{Id: 0}, err
	}
	s.session.NotifyPlayers(Notification{CLIENT_CONNECTED, s.session.GetPlayersName(clientId)}, ALL)
	s.nextClientId++
	if s.session.GetPlayersCount() == PLAYERS_LOWER_LIM {
		s.sessionStart <- 1
	}
	s.mutex.Unlock()
	return &proto.ClientId{Id: clientId}, nil
}

func (s *server) Disconnect(_ context.Context, req *proto.ClientId) (*proto.EmptyMsg, error) {
	s.session.NotifyPlayers(Notification{CLIENT_DISCONNECTED, s.session.GetPlayersName(req.Id)}, ALL)

	s.mutex.Lock()
	s.session.RemovePlayer(req.Id)
	s.mutex.Unlock()
	return &proto.EmptyMsg{}, nil
}

func (s *server) SubscribeToNotifications(req *proto.ClientId, stream proto.Mafia_SubscribeToNotificationsServer) error {
	event, err := s.session.GetPlayersNotifications(req.Id)
	for ; err == nil; event, err = s.session.GetPlayersNotifications(req.Id) {
		switch event.eventType {
		case CLIENT_CONNECTED:
			if err := stream.Send(&proto.Notification{Info: "Player " + event.info + " connected"}); err != nil {
				return err
			}
		case CLIENT_DISCONNECTED:
			if err := stream.Send(&proto.Notification{Info: "Player " + event.info + " disconnected"}); err != nil {
				return err
			}
		case SESSION_DISCLAIMER:
			if err := stream.Send(&proto.Notification{Info: fmt.Sprintf("The game will start in %d seconds", START_DELAY/time.Second)}); err != nil {
				return err
			}
		case SESSION_ABORT:
			if err := stream.Send(&proto.Notification{Info: "There are not enough players to continue game, some of them might have disconnected"}); err != nil {
				return err
			}
		case SESSION_START:
			if err := stream.Send(&proto.Notification{Info: "---- GAME STARTED ----"}); err != nil {
				return err
			}
		case SESSION_END:
			if err := stream.Send(&proto.Notification{Info: "---- GAME ENDED ----\nThe outcome: " + event.info}); err != nil {
				return err
			}
		case ROLE_ASSIGNED:
			if err := stream.Send(&proto.Notification{Info: fmt.Sprintf("You have been assigned the role of: %s", event.info)}); err != nil {
				return err
			}
		case PLAYER_NOT_FOUND:
			if err := stream.Send(&proto.Notification{Info: fmt.Sprintf("There is no player with the name '%s' in the current session", event.info)}); err != nil {
				return err
			}
		case PLAYER_EXPOSED:
			if err := stream.Send(&proto.Notification{Info: fmt.Sprintf("The Detective has found out that '%s' is a member of Mafia!", event.info)}); err != nil {
				return err
			}
		case NO_EXPOSED_PLAYER:
			if err := stream.Send(&proto.Notification{Info: "you haven't exposed anyone during last night"}); err != nil {
				return err
			}
		case GUESS_SUCCESS:
			if err := stream.Send(&proto.Notification{Info: "the selected player is a member of Mafia!"}); err != nil {
				return err
			}
		case GUESS_FAIL:
			if err := stream.Send(&proto.Notification{Info: "the selected player is not a member of Mafia"}); err != nil {
				return err
			}
		case PLAYER_ELIMINATED:
			nameRole := strings.Split(event.info, " ")
			if err := stream.Send(&proto.Notification{Info: fmt.Sprintf("Player '%s' was a %s and has been eliminated. He may continue to observe the game session as a ghost", nameRole[0], nameRole[1])}); err != nil {
				return err
			}
		case VOTING_RESTRICTED:
			if err := stream.Send(&proto.Notification{Info: fmt.Sprintf("Voting is restricted for you: %s", event.info)}); err != nil {
				return err
			}
		case VOTES_MISMATCH:
			if err := stream.Send(&proto.Notification{Info: "There wasn't a single target with the highest count of votes, so no-one is being executed"}); err != nil {
				return err
			}
		case MAFIA_VOTES_MISMATCH:
			if err := stream.Send(&proto.Notification{Info: "All mafia members have to vote for the same person, but there has been a mismatch"}); err != nil {
				return err
			}
		case PHASE_START_DAY:
			if err := stream.Send(&proto.Notification{Info: "---- A new day has started ----"}); err != nil {
				return err
			}
		case PHASE_START_NIGHT:
			if err := stream.Send(&proto.Notification{Info: "---- Darkness falls upon the city... ----"}); err != nil {
				return err
			}
		case CHAT_MSG:
			nameMsg := strings.Split(event.info, "@@")
			if err := stream.Send(&proto.Notification{Info: fmt.Sprintf("%s -> : %s", nameMsg[0], nameMsg[1])}); err != nil {
				return err
			}
		case CHAT_RESTRICTED:
			if err := stream.Send(&proto.Notification{Info: fmt.Sprintf("You can't send message now: %s", event.info)}); err != nil {
				return err
			}
		}
	}

	log.Printf("ClientId %d notification error: %v\n", req.Id, err)
	return nil
}

func (s *server) ShowPlayersList(context.Context, *proto.EmptyMsg) (*proto.PlayersList, error) {
	return &proto.PlayersList{Players: s.session.GetConnectedPlayers()}, nil
}

func (s *server) Vote(_ context.Context, req *proto.ClientReq) (*proto.EmptyMsg, error) {
	s.session.PlayerVote(req.Id.Id, req.Target.Name)
	return &proto.EmptyMsg{}, nil
}

func (s *server) EndDay(_ context.Context, req *proto.ClientId) (*proto.EmptyMsg, error) {
	s.session.PlayerEndDay(req.Id)
	return &proto.EmptyMsg{}, nil
}

func (s *server) Expose(_ context.Context, req *proto.ClientId) (*proto.EmptyMsg, error) {
	s.session.PlayerExpose(req.Id)
	return &proto.EmptyMsg{}, nil
}

func (s *server) Chat(_ context.Context, req *proto.ChatMsg) (*proto.EmptyMsg, error) {
	s.session.SendChatMsg(req.Id.Id, req.Msg)
	return &proto.EmptyMsg{}, nil
}

func (s *server) ObserveSession() {
	for {
		select {
		case <-s.sessionStart:
			for s.session.HasStarted() {
				time.Sleep(START_DELAY)
			}
			// wait for extra players to join before starting game session
			log.Println("Awaiting session start")
			s.session.NotifyPlayers(Notification{eventType: SESSION_DISCLAIMER}, "")
			time.Sleep(START_DELAY)
			s.session.Start()
		}
	}
}

func Run(port int) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	servImpl := server{
		session: &mafiaSession{
			players:              make(map[uint64]MafiaPlayer),
			potentialVictims:     make(map[string]int),
			delayedNotifications: []Notification{},
		},
		nextClientId: 0,
		sessionStart: make(chan int),
	}
	s := grpc.NewServer()
	proto.RegisterMafiaServer(s, &servImpl)
	log.Printf("SERVER listening at %v", listener.Addr())
	go servImpl.ObserveSession()
	if err := s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
