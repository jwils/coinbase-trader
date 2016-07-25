package main

import (
	"fmt"
	"os"
	"time"

	ws "github.com/gorilla/websocket"
	influx "github.com/influxdata/influxdb/client/v2"
	exchange "github.com/preichenberger/go-coinbase-exchange"
)

const (
	database              = "trader"
	coinbaseWebsocketHost = "wss://ws-feed.exchange.coinbase.com"
)

func main() {
	iClient, err := influx.NewHTTPClient(influx.HTTPConfig{
		Addr: "http://localhost:8086"})
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})
	msgs := GetEventMessages()
	go func() {
		bp, err := influx.NewBatchPoints(influx.BatchPointsConfig{
			Database:  database,
			Precision: "s",
		})
			lastTradePrice := 0.0
		for {
			select {
			case m := <-msgs:
				if m.Type != "match" {
					continue
				}
				tags := map[string]string{
					"side": m.Side,
				}
				fields := map[string]interface{}{
					"value": m.Price,
				}
				fmt.Println(m.Price)
				lastTradePrice = m.Price
				pt, err := influx.NewPoint("trades", tags, fields, time.Now())
				bp.AddPoint(pt)
				if err != nil {
					panic(err)
				}
			case <-ticker.C:
				secret := os.Getenv("COINBASE_SECRET")
				key := os.Getenv("COINBASE_KEY")
				passphrase := os.Getenv("COINBASE_PASSPHRASE")

				client := exchange.NewClient(secret, key, passphrase)

				if err != nil {
					panic(err)
				}
				accounts, err := client.GetAccounts()
				if err != nil {
					panic(err.Error())
				}
				total := 0.0
				for _, a := range accounts {
					tags := map[string]string{
						"id":       a.Id,
						"currency": a.Currency,
					}
					fields := map[string]interface{}{
						"value": a.Balance,
					}
					if a.Currency == "USD" {
						total += a.Balance
					}
					if a.Currency == "BTC" {
						fields["usd"] = a.Balance * lastTradePrice
						total += a.Balance * lastTradePrice
					}
					pt, err := influx.NewPoint("balance", tags, fields, time.Now())

					if err != nil {
						panic(err)
					}
					bp.AddPoint(pt)

				}
				tags := map[string]string{
					"id": "all",
					"currency": "sum",
				}
				fields := map[string]interface{}{
					"value": total,
				}
				pt, err := influx.NewPoint("balance", tags, fields, time.Now())

				if err != nil {
					panic(err)
				}
				bp.AddPoint(pt)

				iClient.Write(bp)
				bp, err = influx.NewBatchPoints(influx.BatchPointsConfig{
					Database:  database,
					Precision: "s",
				})
				if err != nil {
					panic(err.Error())
				}
			// dr stuff
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
	select {}
	close(quit)
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
