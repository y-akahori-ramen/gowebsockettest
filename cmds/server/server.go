package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}
var clientsLock sync.Mutex
var clients []*client

type client struct {
	socket *websocket.Conn
}

var join chan *client = make(chan *client)
var leave chan *client = make(chan *client)

func connect(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
	}

	client := &client{socket: c}
	join <- client

	defer func() {
		leave <- client
	}()

	for {
		_, _, err := client.socket.ReadMessage()
		if err != nil {
			log.Printf("クライアント %s からの読み込みに失敗しました", client.socket.LocalAddr())
			break
		}

		log.Printf("クライアント %s から終了通知を受け取りました", client.socket.LocalAddr())
		break
	}
}

func broadcast(w http.ResponseWriter, r *http.Request) {
	clientsLock.Lock()
	for _, client := range clients {
		log.Printf("書き込みます: %s", client.socket.LocalAddr())
		err := client.socket.WriteMessage(websocket.TextMessage, []byte("Broadcast"))
		if err != nil {
			log.Println("書き込みに失敗しました:", err)
		}
	}
	clientsLock.Unlock()
}

func main() {
	http.HandleFunc("/connect", connect)
	http.HandleFunc("/broadcast", broadcast)

	defer func() {
		clientsLock.Lock()
		for _, client := range clients {
			log.Printf("接続を閉じます: %s", client.socket.LocalAddr().String())
			client.socket.Close()
		}
		clientsLock.Unlock()
	}()

	go func() {
		for {
			select {
			case client := <-join:
				log.Printf("connect:%s", client.socket.LocalAddr().String())
				clientsLock.Lock()
				clients = append(clients, client)
				clientsLock.Unlock()
			case leaveClient := <-leave:
				log.Printf("leave:%s", leaveClient.socket.LocalAddr().String())
				clientsLock.Lock()
				result := []*client{}
				for _, client := range clients {
					if leaveClient != client {
						result = append(result, client)
					}
				}
				clients = result
				clientsLock.Unlock()
			}
		}
	}()

	go func() {
		srv := &http.Server{Addr: "localhost:8080"}
		log.Println("サーバー起動します")
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	termSignalChan := make(chan os.Signal, 1)
	signal.Notify(termSignalChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-termSignalChan:
			log.Println("終了します")
			return
		case <-ticker.C:
			clientsLock.Lock()
			for _, client := range clients {
				log.Printf("書き込みます: %s", client.socket.LocalAddr())
				err := client.socket.WriteMessage(websocket.TextMessage, []byte("hello"))
				if err != nil {
					log.Println("書き込みに失敗しました:", err)
				}
			}
			clientsLock.Unlock()
		}
	}
}
