package main

import (
	"net/http"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack/slackevents"
)

var tradingview = NewTradingView()

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetLevel(log.DebugLevel)

	InitRedis(RedisConfig{
		RedisURL: os.Getenv("REDIS_URL"),
		Prefix:   os.Getenv("REDIS_KEY_PREFIX"),
	})
	go Redis.Start()

	slack := &http.Server{
		Handler:      SlackEventRouter(),
		Addr:         os.Getenv("HTTP_SERVER_BIND"),
		WriteTimeout: 1 * time.Minute,
		ReadTimeout:  1 * time.Minute,
	}

	//InitWatchList()
	tradingview.OnConnected = func(tv TradingView) {
		Redis.ForEach(func(user User) {
			source := Command{
				Event: &slackevents.MessageEvent{
					Channel: os.Getenv("SLACK_DEFAULT_CHANNEL"),
				},
				User: &user,
			}
			for i := range user.Portfolio {
				asset := user.Portfolio[i]
				tv.Watch(asset.Symbol)

				if asset.Type == "limit_buy" || asset.Type == "limit_sell" {
					user.WatchLimitOrder(asset, &source)
				}
			}
		})
	}
	go tradingview.Connect()

	log.Fatal(slack.ListenAndServe())
}
