package main

import (
	"encoding/json"
	"sync"

	"github.com/go-redis/redis/v8"
)

type RedisClient struct {
	client  *redis.Client
	quit    chan struct{}
	started bool
	limit   int
	mutex   *sync.Mutex
	prefix  string
}

type RedisConfig struct {
	RedisURL string
	Prefix   string
}

type TradingViewRequest struct {
	Method string        `json:"m"`
	Params []interface{} `json:"p"`
}

type TradingViewEvent struct {
	Type      string            `json:"m"`
	RawData   []json.RawMessage `json:"p"`
	SessionID string
	Data      interface{}
}

type TradingViewEventQSDEnvelope struct {
	Symbol string          `json:"n"`
	Status string          `json:"s"`
	Data   json.RawMessage `json:"v"`
}

type TradingViewQuote struct {
	Symbol               string  `json:"short_name"`
	FullName             string  `json:"description"`
	CurrencyCode         string  `json:"currency_code"`
	IsTradable           bool    `json:"is_tradable"`
	Exchange             string  `json:"listed_exchange"`
	OriginalName	     string  `json:"original_name"`
	ProName		     string  `json:"pro_name"`
	CurrentSession       string  `json:"current_session"`
	LastPrice            float64 `json:"lp"`
	Change               float64 `json:"ch"`
	ChangePercentage     float64 `json:"chp"`
	LivePrice            float64 `json:"rtc"`
	LiveChange           float64 `json:"rch"`
	LiveChangePercentage float64 `json:"rchp"`
}

type TradingViewEventTimescaleUpdateEnvelope struct {
	Record TradingViewEventTimescaleUpdateSeries `json:"s1"`
}

type TradingViewEventTimescaleUpdateSeries struct {
	Node string                                  `json:"node"`
	Data []TradingViewEventTimescaleUpdateRecord `json:"s"`
}

type TradingViewEventTimescaleUpdateRecord struct {
	Index  int       `json:"i"`
	Values []float64 `json:"v"`
}
