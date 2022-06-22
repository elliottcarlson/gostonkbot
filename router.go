package main

import (
	"github.com/gorilla/mux"
)

func Router() *mux.Router {
	router := mux.NewRouter()

	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/funds", HandleFunds)
	api.HandleFunc("/portfolio", HandlePortfolio)
	api.HandleFunc("/leaderboard", HandleLeaderBoard)

	router.HandleFunc("/slack/events", SlackEventHandler)

	return router
}
