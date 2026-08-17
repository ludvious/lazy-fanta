package model

import (
	"lazy-fanta/internal/costants"
	"time"
)

// Coach represents a partecipant in the auction and fantasyleague
type Coach struct {
	Name    string   `json:"name"`
	Info    Info     `json:"info"`
	Players []Player `json:"players"`
}

type Player struct {
	Name string        `json:"name"`
	Role costants.Role `json:"role"`
	Cost int           `json:"cost"`
}

// Purchase represents a purchase made by a user.
type Purchase struct {
	ID         int           `json:"id"`
	CoachName  string        `json:"coach"`
	PlayerName string        `json:"player_name"`
	PlayerRole costants.Role `json:"player_role"`
	Cost       int           `json:"cost"`
	Timestamp  time.Time     `json:"timestamp"`
}

type Info struct {
	InitialBudget int `json:"initial_budget"`
	Budget        int `json:"budget"`
	MaxBid        int `json:"max_bid"`
	TotalSpent    int `json:"total_spent"`
}

type Session struct {
	Filepath    string     `json:"filepath"`
	ID          string     `json:"session_id"`
	AuctionName string     `json:"auction_name"`
	UpdatedAt   string     `json:"updated_at"`
	Status      string     `json:"status"`
	Purchases   []Purchase `json:"purchases"`
	Coaches     []Coach    `json:"coaches"`
}
