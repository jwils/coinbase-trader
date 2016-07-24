package main

import (
	"encoding/json"
	"fmt"
	"github.com/HuKeping/rbtree"
	"os"

	exchange "github.com/preichenberger/go-coinbase-exchange"
	"math"
	"time"

	influx "github.com/influxdata/influxdb/client/v2"
)

const (
	database = "trader"
)

type LimitOrderEntry struct {
	OrderId string
	Price   float64
	Size    float64
	Type    string
}

type PriceEntry struct {
	Price        float64
	OrderEntries map[string]*LimitOrderEntry
}

func (p1 PriceEntry) Less(p2 rbtree.Item) bool {
	return p1.Price < p2.(*PriceEntry).Price
}

type OrderBook struct {
	OrderLookup map[string]*LimitOrderEntry
	BuyEntries  *rbtree.Rbtree
	SellEntries *rbtree.Rbtree
}

func (o OrderBook) AddBuy(li *LimitOrderEntry) {
	o.OrderLookup[li.OrderId] = li
	buyPrice := o.BuyEntries.Get(&PriceEntry{li.Price, nil})
	if buyPrice == nil {
		buyPrice = &PriceEntry{li.Price, make(map[string]*LimitOrderEntry)}
		o.BuyEntries.Insert(buyPrice)
	}
	buyPrice.(*PriceEntry).OrderEntries[li.OrderId] = li
}

func (o OrderBook) DeleteOrder(orderId string) {
	orderEntry := o.OrderLookup[orderId]
	var priceEntry *PriceEntry
	var ok bool
	if orderEntry == nil {
		println("Order:" + orderId + " not found.")
		return
	}
	if orderEntry.Type == "buy" {
		priceEntry, ok = o.BuyEntries.Get(&PriceEntry{orderEntry.Price, nil}).(*PriceEntry)
		if !ok {
			println("Failed to find buy order " + orderId)
			return
		}
	} else {
		priceEntry, ok = o.SellEntries.Get(&PriceEntry{orderEntry.Price, nil}).(*PriceEntry)
		if !ok {
			println("Failed to find sell order " + orderId)
			return
		}
	}
	delete(priceEntry.OrderEntries, orderId)
	if len(priceEntry.OrderEntries) == 0 {
		if orderEntry.Type == "buy" {
			o.BuyEntries.Delete(priceEntry)
		} else {
			o.SellEntries.Delete(priceEntry)

		}
	}
	delete(o.OrderLookup, orderId)
}

func (o OrderBook) AddSell(li *LimitOrderEntry) {
	o.OrderLookup[li.OrderId] = li
	buyPrice := o.SellEntries.Get(&PriceEntry{li.Price, nil})
	if buyPrice == nil {
		buyPrice = &PriceEntry{li.Price, make(map[string]*LimitOrderEntry)}
		o.SellEntries.Insert(buyPrice)
	}
	buyPrice.(*PriceEntry).OrderEntries[li.OrderId] = li
}

func NewOrderBook() *OrderBook {
	orders := make(map[string]*LimitOrderEntry)
	return &OrderBook{orders, rbtree.New(), rbtree.New()}
}

func Print(item rbtree.Item) bool {
	i, ok := item.(*PriceEntry)
	if !ok {
		return true
	}
	fmt.Printf("%v Order Count: %v\n", i.Price, len(i.OrderEntries))
	return true
}

func Round(f float64) float64 {
	return math.Floor(f + .5)
}

func RoundPlus(f float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return Round(f*shift) / shift
}

func main() {
	o := NewOrderBook()
	socketMessages := GetEventMessages()
	//Make sure we get our first socket response before fetching the book snapshot
	<-socketMessages
	ob := GetCoinbaseOrderBook()

	for _, entry := range ob.Bids {
		o.AddBuy(&LimitOrderEntry{entry.OrderId, entry.Price, entry.Size, "buy"})
	}

	for _, entry := range ob.Asks {
		o.AddSell(&LimitOrderEntry{entry.OrderId, entry.Price, entry.Size, "sell"})
	}

	//fmt.Printf("%d\n", o.BuyEntries)
	o.BuyEntries.Ascend(&PriceEntry{380, nil}, Print)

	lastSeq := 0

	iClient, err := influx.NewHTTPClient(influx.HTTPConfig{
		Addr: "http://localhost:8086"})
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				secret := os.Getenv("COINBASE_SECRET")
				key := os.Getenv("COINBASE_KEY")
				passphrase := os.Getenv("COINBASE_PASSPHRASE")

				client := exchange.NewClient(secret, key, passphrase)
				bp, err := influx.NewBatchPoints(influx.BatchPointsConfig{
					Database:  database,
					Precision: "s",
				})
				if err != nil {
					panic(err)
				}
				accounts, err := client.GetAccounts()
				if err != nil {
					panic(err.Error())
				}
				for _, a := range accounts {
					tags := map[string]string{
						"id": a.Id,
						"currency": a.Currency,
					}
					fields := map[string]interface{}{
						"value": a.Balance,
					}
					pt, err := influx.NewPoint("balance", tags, fields, time.Now())
					if err != nil {
						panic(err)
					}
					bp.AddPoint(pt)
				//	cursor := client.ListAccountLedger(a.Id)
  				//	var ledger []exchange.LedgerEntry
				//
				//	for cursor.HasMore {
				//		if err := cursor.NextPage(&ledger); err != nil {
				//			panic(err.Error())
				//		}

//						for _, e := range ledger {
//							fmt.Println(e.Amount)
//						}
//					}
				}
				iClient.Write(bp)

				// dr stuff
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	f := func() {
		secret := os.Getenv("COINBASE_SECRET")
		key := os.Getenv("COINBASE_KEY")
		passphrase := os.Getenv("COINBASE_PASSPHRASE")

		client := exchange.NewClient(secret, key, passphrase)

		time.Sleep(time.Second * 5)
		for {
			fmt.Println("KEY:" + key)
			currentMaxBuy := o.BuyEntries.Max().(*PriceEntry).Price
			currentMinSell := o.SellEntries.Min().(*PriceEntry).Price

			buyPrice := RoundPlus(currentMaxBuy-1.80, 2)

			fmt.Printf("buy Price: %v", buyPrice)

			sellPrice := RoundPlus(currentMinSell+1.80, 2)

			fmt.Println("sell price: %v", sellPrice)

			o1, err := client.CreateOrder(&exchange.Order{
				Price:     buyPrice,
				Size:      0.01,
				Side:      "buy",
				ProductId: "BTC-USD",
			})

			if err != nil {
				panic(err)
			}

			o2, err := client.CreateOrder(&exchange.Order{
				Price:     sellPrice,
				Size:      0.01,
				Side:      "sell",
				ProductId: "BTC-USD",
			})

			if err != nil {
				panic(err)
			}

			time.Sleep(time.Second * 15)

			err = client.CancelOrder(o1.Id)
			if err != nil && err.Error() != "Order already done" {
				panic(err)
			}
			err = client.CancelOrder(o2.Id)
			if err != nil && err.Error() != "Order already done" {
				panic(err)
			}

		}
	}
	//go f()
	fmt.Printf("%v", f)


	for n := range socketMessages {
		if n.Sequence <= ob.Sequence {
			lastSeq = n.Sequence
			continue
		}
		if lastSeq+1 != n.Sequence {
			println("Missing some entries")
		}
		switch n.Type {
		case "open":
			//Ignore for now
		case "received":
			if n.Side == "buy" && n.OrderType == "limit" {
				//currentMax := o.BuyEntries.Max().(*PriceEntry).Price
				o.AddBuy(&LimitOrderEntry{n.OrderId, n.Price, n.Size, "buy"})
			} else if n.Side == "sell" && n.OrderType == "limit" {
				o.AddSell(&LimitOrderEntry{n.OrderId, n.Price, n.Size, "sell"})
			} else {
				//println(n.OrderId)
			}
			//currentMaxBuy := o.BuyEntries.Max().(*PriceEntry).Price
			//currentMinSell := o.SellEntries.Min().(*PriceEntry).Price
			//fmt.Printf("Mid price: %0.2f\n", (currentMaxBuy+currentMinSell)/2)

		case "done":
			//println(n.OrderId)
			o.DeleteOrder(n.OrderId)
		case "match":
			bp, err := influx.NewBatchPoints(influx.BatchPointsConfig{
				Database:  database,
				Precision: "s",
			})
			if err != nil {
				panic(err)
			}
			fmt.Printf("Sale at %v\n", n)
			tags := map[string]string{}
			fields := map[string]interface{}{
				"value": n.Price,
			}
			pt, err := influx.NewPoint("price", tags, fields, time.Now())
			if err != nil {
				panic(err)
			}
			bp.AddPoint(pt)
			iClient.Write(bp)
		default:
			v, _ := json.Marshal(n)
			fmt.Printf("%v\n", string(v))

		}
		lastSeq = n.Sequence
	}

	close(quit)
}
