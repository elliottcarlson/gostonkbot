package main

import (
	"sort"
)

type LeaderBoardEntry struct {
	Name     string  `json:"name"`
	NetWorth float64 `json:"networth"`
}

func GetLeaderboard() (leaderboard []LeaderBoardEntry) {
	users := Redis.GetAllUsers()

	for i := range users {
		user := users[i]
		leaderboard = append(leaderboard, LeaderBoardEntry{
			Name:     user.FullName,
			NetWorth: user.NetWorth(),
		})
	}

	sort.Slice(leaderboard[:], func(i, j int) bool {
		return leaderboard[i].NetWorth > leaderboard[j].NetWorth
	})

	return leaderboard
}
