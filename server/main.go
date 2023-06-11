package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"google.golang.org/grpc"

	pb "proto.pb" // Импортируйте ваш пакет сгенерированного протокола
)

// Реализация службы
type myServiceServer struct {
	mu      sync.Mutex
	clients []pb.MyService_ConnectServer
}

func (s *myServiceServer) Connect(stream pb.MyService_ConnectServer) error {
	s.mu.Lock()
	s.clients = append(s.clients, stream)
	s.mu.Unlock()

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// Обработать входящий запрос от клиента
		fmt.Println("Получено сообщение от клиента:", req.Message)

		// Отправить ответ обратно клиенту
		res := &pb.Response{Message: "Привет, клиент!"}
		for _, client := range s.clients {
			err := client.Send(res)
			if err != nil {
				log.Printf("Ошибка при отправке ответа клиенту: %v", err)
			}
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Ошибка при прослушивании порта: %v", err)
	}

	// Создать gRPC сервер
	server := grpc.NewServer()

	// Зарегистрировать службу на сервере
	pb.RegisterMyServiceServer(server, &myServiceServer{})

	log.Println("Сервер запущен...")

	// Запустить сервер
	if err := server.Serve(lis); err != nil {
		log.Fatalf("Ошибка при запуске сервера: %v", err)
	}
}
