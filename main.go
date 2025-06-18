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
	symbol       = "/NQ" // change to "/ES" if needed
	quantity     = 10
	maxDailyLoss = -800.0
	minTradeGap  = 180 // seconds between trades
)

var (
	accountID  = os.Getenv("ACCOUNT_ID")
	secretKey  = os.Getenv("API_KEY")
	lastTrade  time.Time
	totalPnL   float64
	mutex      sync.Mutex
)

func placeOrder(side string) {
	mutex.Lock()
	defer mutex.Unlock()

	if time.Since(lastTrade).Seconds() < minTradeGap {
		log.Println("Trade skipped: too soon since last trade")
		return
	}

	if totalPnL <= maxDailyLoss {
		log.Println("Trading halted: max daily loss reached")
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

	bodyData, _ := json.Marshal(order)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(bodyData))
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

	respBody, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("Trade Executed:", string(respBody))

	// Simulated PnL update (mock values)
	mockPnL := 250.0
	if side == "sell" {
		mockPnL = -150.0
	}
	totalPnL += mockPnL
	lastTrade = time.Now()
	log.Printf("Total PnL: $%.2f\n", totalPnL)
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
		log.Println("Unknown signal received:", signal.Message)
	}
}

func main() {
	http.HandleFunc("/webhook", webhookHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Println("Bot listening on port " + port)
	http.ListenAndServe(":"+port, nil)
}
