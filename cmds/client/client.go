package main

import (
	"context"
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
)

func main() {
	var addr = flag.String("addr", "localhost:8080", "接続先サーバーアドレス")
	flag.Parse()

	url := url.URL{Scheme: "ws", Host: *addr, Path: "/connect"}
	log.Printf("%s へ接続します", url.String())

	con, _, err := websocket.DefaultDialer.Dial(url.String(), nil)
	if err != nil {
		log.Fatal("接続に失敗しました:", err)
	}
	defer con.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				con.WriteMessage(websocket.TextMessage, []byte("leave"))
				log.Print("読み込みループを終了します")
				return
			default:
				_, message, err := con.ReadMessage()
				if err != nil {
					log.Print("読み込みに失敗しました:", err)
					return
				}
				log.Printf("読み込みました: %s", message)
			}
		}
	}()

	termSignalChan := make(chan os.Signal, 1)
	signal.Notify(termSignalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-termSignalChan:
			log.Println("終了します")
			cancel()
		case <-done:
			return
		}
	}
}
