package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type Signal struct {
	Message string `json:"message"`
}

type OrderRequest struct {
	AccountID   string `json:"account_id"`
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	Quantity    int    `json:"quantity"`
	OrderType   string `json:"order_type"`
	TimeInForce string `json:"time_in_force"`
}

const (
	apiURL       = "https://api.topstep.com/v1/order"
	secretKey    = "aPscKa0MbL0kMu0USY/UO3i3Q4cT+841+VOGeJqMddo="
	accountID    = "50KTC-V2-111386-61492234"
	symbol       = "/NQ" // use "/ES" if you want to trade ES instead
	quantity     = 10
	maxDailyLoss = -800.0
	minTradeGap  = 180 // seconds
)

var (
	lastTradeTime time.Time
	totalProfit   float64
	mutex         sync.Mutex
)

func placeOrder(side string) {
	mutex.Lock()
	defer mutex.Unlock()

	if time.Since(lastTradeTime).Seconds() < minTradeGap {
		log.Println("Trade skipped: throttle active")
		return
	}

	if totalProfit <= maxDailyLoss {
		log.Println("Max daily loss reached. Trading halted.")
		return
	}

	order := OrderRequest{
		AccountID:   accountID,
		Symbol:      symbol,
		Side:        side,
		Quantity:    quantity,
		OrderType:   "market",
		TimeInForce: "GTC",
	}
	jsonOrder, _ := json.Marshal(order)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonOrder))
	if err != nil {
		log.Println("Request error:", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("API call failed:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("Trade Executed:", string(body))

	mockPnL := 250.0
	if side == "sell" {
		mockPnL = -150.0
	}
	totalProfit += mockPnL
	lastTradeTime = time.Now()
	log.Printf("Updated PnL: $%.2f\n", totalProfit)
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	var signal Signal
	err := json.NewDecoder(r.Body).Decode(&signal)
	if err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if signal.Message == "BUY_SIGNAL" {
		placeOrder("buy")
	} else if signal.Message == "SELL_SIGNAL" {
		placeOrder("sell")
	} else {
		log.Println("Unknown signal received:", signal
