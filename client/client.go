package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"mafia-core/proto"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type client struct {
	dialer      proto.MafiaClient
	id          uint64
	conn        *grpc.ClientConn
	isConnected bool
}

var cl = client{isConnected: false}

func (c *client) checkState() bool {
	if !c.isConnected {
		fmt.Println("You are not connected to a game session, join a server first")
	}

	return c.isConnected
}

// TODO: add empty string check for name
func (c *client) Connect(clientName, address string) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Couldn't connect to grpc server: %v\n", err)
		return
	}

	c.conn = conn
	c.dialer = proto.NewMafiaClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	assignedId, err := cl.dialer.Connect(ctx, &proto.ClientInfo{Name: clientName})
	if err != nil {
		if err := cl.conn.Close(); err != nil {
		}
		log.Printf("Couldn't connect to server: %v\n", err.Error())
		return
	}
	c.id = assignedId.Id
	c.isConnected = true
}

func (c *client) Disconnect() {
	if !c.checkState() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := c.dialer.Disconnect(ctx, &proto.ClientId{Id: c.id})
	if err != nil {
		log.Printf("Error while Disconnecting: %v\n", err)
	}

	if err := cl.conn.Close(); err != nil {
	}

	c.isConnected = false
}

func (c *client) Chat(msg string) {
	if !c.checkState() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := c.dialer.Chat(ctx, &proto.ChatMsg{Id: &proto.ClientId{Id: c.id}, Msg: msg})
	if err != nil {
		log.Printf("Couldn't get response from server: %v\n", err)
	}
}

func (c *client) Subscribe() {
	if !c.checkState() {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := c.dialer.SubscribeToNotifications(ctx, &proto.ClientId{Id: c.id})
	if err != nil {
		log.Println("Subscription Failed")
		cl.Disconnect()
		return
	}

	for {
		notification, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("Stopped receiving notifications from server, try reconnecting")
			break
		}
		log.Printf(notification.Info)
		if len(notification.Info) > 12 && notification.Info[:12] == "The outcome" {
			break
		}
	}

	if cl.isConnected {
		cl.Disconnect()
	}
}

func (c *client) ShowPlayersList() {
	if !c.checkState() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := c.dialer.ShowPlayersList(ctx, &proto.EmptyMsg{})
	if err != nil {
		log.Printf("Couldn't get response from server: %v\n", err)
	}

	fmt.Println("Players in session:")
	for _, name := range resp.Players {
		fmt.Println(name)
	}
}

func (c *client) Vote(target string) {
	if !c.checkState() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := c.dialer.Vote(ctx, &proto.ClientReq{Id: &proto.ClientId{Id: c.id}, Target: &proto.ClientInfo{Name: target}})
	if err != nil {
		log.Printf("Couldn't get response from server: %v\n", err)
	}
}

func (c *client) EndDay() {
	if !c.checkState() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := c.dialer.EndDay(ctx, &proto.ClientId{Id: c.id})
	if err != nil {
		log.Printf("Couldn't get response from server: %v\n", err)
	}
}

func (c *client) Expose() {
	if !c.checkState() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := c.dialer.Expose(ctx, &proto.ClientId{Id: c.id})
	if err != nil {
		log.Printf("Couldn't get response from server: %v\n", err)
	}
}

func Run() {
	defer cl.Disconnect()

	fmt.Println("----\tYou have launched Mafia client\t----\nprint 'help' for the list of available commands")
	for reader := bufio.NewReader(os.Stdin); ; {
		cmd, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading string", err)
			continue
		}

		switch parseCommand(strings.TrimSpace(cmd)) {
		case CONNECT:
			if cl.isConnected {
				fmt.Println("You are already in the game session")
				break
			}

			fmt.Println("Enter server's address:")
			serverAddr, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Error parsing server address", err)
				continue
			}

			fmt.Println("Enter your nickname:")
			name, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Error parsing nickname", err)
				continue
			}

			cl.Connect(strings.TrimSpace(name), strings.TrimSpace(serverAddr))
			go cl.Subscribe()
		case DISCONNECT:
			cl.Disconnect()
		case SHOW_PLAYER_LIST:
			cl.ShowPlayersList()
		case VOTE:
			fmt.Println("Enter a player's name:")
			target, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Error parsing player's name to vote", err)
				continue
			}
			cl.Vote(strings.TrimSpace(target))
		case END_DAY:
			cl.EndDay()
		case EXPOSE:
			cl.Expose()
		case EXIT:
			fmt.Println("Bye-bye!")
			cl.Disconnect()
			os.Exit(0)
		case CHAT:
			fmt.Println("Enter your message:")
			msg, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Error reading message", err)
				continue
			}

			cl.Chat(strings.TrimSpace(msg))
		case HELP:
			showHints()
		case UNKNOWN:
			fmt.Println("Unknown command, print 'help' to see available commands")
		}

	}
}
