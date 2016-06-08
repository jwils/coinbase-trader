package main

import (
	//"encoding/json"
	ws "github.com/gorilla/websocket"
	exchange "github.com/preichenberger/go-coinbase-exchange"
	"os"
)

const (
	coinbaseWebsocketHost = "wss://ws-feed.exchange.coinbase.com"
)

func GetCoinbaseOrderBook() exchange.Book {
	secret := os.Getenv("COINBASE_SECRET")
	key := os.Getenv("COINBASE_KEY")
	passphrase := os.Getenv("COINBASE_PASSPHRASE")
	client := exchange.NewClient(secret, key, passphrase)

	book, _ := client.GetBook("BTC-USD", 3)

	return book
}

func GetEventMessages() <-chan *exchange.Message {
	var wsDialer ws.Dialer
	wsConn, _, err := wsDialer.Dial(coinbaseWebsocketHost, nil)
	if err != nil {
		println(err.Error())
	}

	subscribe := map[string]string{
		"type":       "subscribe",
		"product_id": "BTC-USD",
	}

	if err := wsConn.WriteJSON(subscribe); err != nil {
		println(err.Error())
	}

	socketMessages := make(chan *exchange.Message)
	go func() {
		for true {
			message := &exchange.Message{}
			if err := wsConn.ReadJSON(message); err != nil {
				println(err.Error())
				break
			}
			socketMessages <- message
		}
	}()
	return socketMessages
}

