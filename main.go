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
	Message string  `json:"message"`
	RSI     float64 `json:"rsi"`
	VWAP    float64 `json:"vwap"`
	Price   float64 `json:"price"`
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
	apiURL       = "https://api.topstepx.com/api/Order/place"
	symbol       = "/NQ" // or "/ES"
	quantity     = 10
	maxDailyLoss = -800.0
	minTradeGap  = 180 // seconds between trades
	exitRSI      = 55.0
	entryRSIBuy  = 62.90
	entryRSISell = 37.10
)

var (
	secretKey    = os.Getenv("TOPSTEP_API_KEY")
	accountID    = os.Getenv("TOPSTEP_ACCOUNT_ID")
	lastTradeTime time.Time
	totalProfit float64
	mutex       sync.Mutex
	inPosition  string // "long", "short", or ""
)

func placeOrder(side string) {
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

func exitPosition() {
	log.Println("Exiting position...")
	if inPosition == "long" {
		placeOrder("sell")
	} else if inPosition == "short" {
		placeOrder("buy")
	}
	inPosition = ""
}

func handleSignal(msg string, rsi float64) {
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

	switch msg {
	case "BUY_SIGNAL":
		if inPosition == "" && rsi > entryRSIBuy {
			placeOrder("buy")
			inPosition = "long"
		} else if inPosition == "short" && rsi > entryRSIBuy {
			exitPosition()
			placeOrder("buy")
			inPosition = "long"
		}
	case "SELL_SIGNAL":
		if inPosition == "" && rsi < entryRSISell {
			placeOrder("sell")
			inPosition = "short"
		} else if inPosition == "long" && rsi < entryRSISell {
			exitPosition()
			placeOrder("sell")
			inPosition = "short"
		}
	case "EXIT_SIGNAL":
		exitPosition()
	default:
		log.Println("Unknown signal received:", msg)
	}

	// Auto-exit logic
	if inPosition == "long" && rsi < exitRSI {
		log.Println("Exiting long position due to RSI drop")
		exitPosition()
	} else if inPosition == "short" && rsi > (100 - exitRSI) {
		log.Println("Exiting short position due to RSI rise")
		exitPosition()
	}
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	var signal Signal
	err := json.NewDecoder(r.Body).Decode(&signal)
	if err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	handleSignal(signal.Message, signal.RSI)
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
