package main

import (
	"testing"
	//"bufio"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

func BenchmarkOrderBookProcessingTime(*testing.B) {
	o := NewOrderBook()
	f, _ := os.Open("book.dat")
	book := &Book{}
	decoder := json.NewDecoder(f)
	decoder.Decode(&book)

	for _, entry := range book.Bids {
		o.AddBuy(&LimitOrderEntry{entry.OrderId, entry.Price, entry.Size, "buy"})
	}

	for _, entry := range book.Asks {
		o.AddSell(&LimitOrderEntry{entry.OrderId, entry.Price, entry.Size, "sell"})
	}
}

type BookEntry struct {
	Price          float64
	Size           float64
	NumberOfOrders int
	OrderId        string
}

type Book struct {
	Sequence int         `json:"sequence"`
	Bids     []BookEntry `json:"bids"`
	Asks     []BookEntry `json:"asks"`
}

type Message struct {
	Type          string  `json:"type"`
	TradeId       int     `json:"trade_id,number"`
	OrderId       string  `json:"order_id"`
	Sequence      int     `json:"sequence,number"`
	MakerOrderId  string  `json:"maker_order_id"`
	TakerOrderId  string  `json:"taker_order_id"`
	RemainingSize float64 `json:"remaining_size,string"`
	NewSize       float64 `json:"new_size,string"`
	OldSize       float64 `json:"old_size,string"`
	Size          float64 `json:"size,string"`
	Price         float64 `json:"price,string"`
	Side          string  `json:"side"`
	Reason        string  `json:"reason"`
	OrderType     string  `json:"order_type"`
	Funds         float64 `json:"funds,string"`
	NewFunds      float64 `json:"new_funds,string"`
	OldFunds      float64 `json:"old_funds,string"`
}

func BenchmarkLiveOrderProcessing(*testing.B) {
	o := NewOrderBook()
	f, _ := os.Open("book.dat")
	book := &Book{}
	decoder := json.NewDecoder(f)
	decoder.Decode(&book)

	for _, entry := range book.Bids {
		o.AddBuy(&LimitOrderEntry{entry.OrderId, entry.Price, entry.Size, "buy"})
	}

	for _, entry := range book.Asks {
		o.AddSell(&LimitOrderEntry{entry.OrderId, entry.Price, entry.Size, "sell"})
	}
	f2, _ := os.Open("feed.dat")
	defer f2.Close()

	r := bufio.NewReader(f2)
	lastSeq := book.Sequence
	for i := 0; i < 121117; i++ {
		b, _ := r.ReadBytes('\n')

		n := &Message{}
		e := json.Unmarshal(b, &n)
		if e != nil {
			fmt.Printf("line %v\n", i)
			panic(e)
			if lastSeq+1 != n.Sequence {
				println("Error")
			}
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
				// v, _ := json.Marshal(n)
				// fmt.Printf("%v\n", string(v))
			}
			//currentMaxBuy := o.BuyEntries.Max().(*PriceEntry).Price
			//currentMinSell := o.SellEntries.Min().(*PriceEntry).Price
			//fmt.Printf("Mid price: %0.2f\n", (currentMaxBuy+currentMinSell)/2)
		case "done":
			//println(n.OrderId)
			o.DeleteOrder(n.OrderId)
		case "match":
			//fmt.Printf("Sale at %v\n", n.Price)
		default:
			v, _ := json.Marshal(n)
			fmt.Printf("%v\n", string(v))

		}
	}
}
