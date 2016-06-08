package main

import (
	"encoding/json"
	"fmt"
	"github.com/HuKeping/rbtree"
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

	fmt.Printf("%d\n", o.BuyEntries)
	o.BuyEntries.Ascend(&PriceEntry{380, nil}, Print)

	lastSeq := 0
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
			currentMaxBuy := o.BuyEntries.Max().(*PriceEntry).Price
			currentMinSell := o.SellEntries.Min().(*PriceEntry).Price
			fmt.Printf("Mid price: %0.2f\n", (currentMaxBuy+currentMinSell)/2)
		case "done":
			//println(n.OrderId)
			o.DeleteOrder(n.OrderId)
		case "match":
			fmt.Printf("Sale at %v\n", n)
		default:
			v, _ := json.Marshal(n)
			fmt.Printf("%v\n", string(v))

		}
		lastSeq = n.Sequence
	}

}
