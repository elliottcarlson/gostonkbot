package main

import (
	"encoding/json"
	"net/http"
)

func HandleLeaderBoard(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(GetLeaderboard())

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}
