package main

import (
	"math"
	"strings"
)

var DEFAULT_WALLET_VALUE = float64(100000)

type User struct {
	UserID    string
	FullName  string
	Funds     float64
	HeldFunds float64
	Portfolio []*Asset
}

type Asset struct {
	Type      string
	Symbol    string
	CostBasis float64
	Quantity  int
}

func GetUserByID(userID string) *User {
	if user, err := Redis.Get(userID); err == nil {
		if user.FullName == "" {
			slackuser, _ := slackapi.GetUserInfo(userID)
			user.FullName = slackuser.Profile.RealName
			user.Save()
		}
		return user
	} else {
		slackuser, _ := slackapi.GetUserInfo(userID)
		user := &User{
			UserID: userID,
			FullName: slackuser.Profile.RealName,
			Funds:  DEFAULT_WALLET_VALUE,
		}

		Redis.Set(userID, user)
		return user
	}
}

func (u *User) Save() {
	Redis.Set(u.UserID, u)
}

func (u *User) CreatePosition(position_type string, symbol string, quantity int64, target float64, source *Command) {
	user := u
	watchlist.GetQuote(symbol, func(quote TradingViewQuote) (shouldDelete bool) {
		user = GetUserByID(user.UserID)
		cost_basis := quote.LastPrice
		if strings.HasPrefix(position_type, "limit_") {
			cost_basis = target
		}

		if quote.Symbol != symbol {
			source.Say("<@s> I was unable to find that stock; wanna try that again?")
			return true
		}

		if position_type != "limit_sell" {
			if cost_basis*float64(quantity) > user.Funds {
				source.Say("<@%s>, you don't have enough funds to cover this trade. You have $%.2f available, and at most could do %d shares.", user.UserID, user.Funds, int(math.Floor(user.Funds/cost_basis)))
				return true
			}
		}

		asset := &Asset{
			Type:      position_type,
			Symbol:    quote.Symbol,
			CostBasis: cost_basis,
			Quantity:  int(quantity),
		}

		user.Portfolio = append(user.Portfolio, asset)
		if position_type != "limit_sell" {
			user.Funds = user.Funds - (cost_basis * float64(quantity))
		}

		var action string
		switch position_type {
		case "long":
			action = "bought"
		case "short":
			action = "shorted"
		case "limit_buy":
			action = "created a limit order to buy"
			user.HeldFunds = user.HeldFunds + (cost_basis * float64(quantity))
			user.WatchLimitOrder(asset, source)
		case "limit_sell":
			action = "created a limit order to sell"
			user.WatchLimitOrder(asset, source)
		case "limit_cover":
			action = "created a limit order to cover"
			user.HeldFunds = user.HeldFunds + (cost_basis * float64(quantity))
			user.WatchLimitOrder(asset, source)
		}

		user.Save()

		source.Say("<@%s> %s %d shares of %s at $%.2f, totalling $%.2f. They have $%.2f funds remaining.", user.UserID, action, quantity, symbol, cost_basis, cost_basis*float64(quantity), user.Funds)
		return true
	})
}

func (u *User) ClosePosition(position_type string, symbol string, quantity int64, basis float64, source *Command) {
	user := u
	watchlist.GetQuote(symbol, func(quote TradingViewQuote) (shouldDelete bool) {
		user = GetUserByID(user.UserID)
		if quote.Symbol != symbol {
			source.Say("<@s> I was unable to find that stock; wanna try that again?")
			return true
		}

		var gains float64
		var funds float64
		var sold int

		var new_portfolio []*Asset
		for i := range user.Portfolio {
			asset := user.Portfolio[i]
			if asset.Symbol == symbol && asset.Type == position_type {
				if (basis != 0 && basis == asset.CostBasis) || basis == 0 {
					to_sell := math.Min(float64(quantity), float64(asset.Quantity))

					proceeds := asset.CostBasis * float64(to_sell)
					value := quote.LastPrice * float64(to_sell)

					switch position_type {
					case "long":
						gains = gains + (value - proceeds)
						funds = funds + value
					case "short":
						gains = gains + (proceeds - value)
						funds = funds + proceeds + gains
					case "limit_buy":
						refund := float64(to_sell) * asset.CostBasis
						user.HeldFunds = user.HeldFunds - refund
						user.Funds = user.Funds + refund
					case "limit_cover":
						refund := float64(to_sell) * asset.CostBasis
						user.HeldFunds = user.HeldFunds - refund
						user.Funds = user.Funds + refund
					}

					quantity = quantity - int64(to_sell)
					asset.Quantity = asset.Quantity - int(to_sell)
					sold = sold + int(to_sell)
				}
			}

			if asset.Quantity > 0 {
				new_portfolio = append(new_portfolio, asset)
			}
		}

		if sold == 0 {
			source.Say("<@%s>, you don't have those shares, are you trying to pull something?", user.UserID)
			return true
		}

		user.Funds = user.Funds + funds
		user.Portfolio = new_portfolio
		user.Save()

		var description string
		switch position_type {
		case "long":
			description = " sold"
		case "short":
			description = " covered"
		case "limit_buy":
			return true
		case "limit_sell":
			return true
		case "limit_cover":
			return true
		}

		source.Say("<@%s>%s %d shares of %s at $%.2f, totalling $%.2f, netting them $%.2f. They have $%.2f funds remaining.", user.UserID, description, sold, symbol, quote.LastPrice, funds, gains, user.Funds)
		return true
	})
}

func (u *User) WatchLimitOrder(order *Asset, source *Command) {
	user := u
	watchlist.OnUpdate(order.Symbol, func(quote TradingViewQuote) (shouldDelete bool) {
		user = GetUserByID(user.UserID)

		asset_found := false
		for i := range user.Portfolio {
			asset := user.Portfolio[i]

			if asset.Type == order.Type && asset.Symbol == order.Symbol && asset.Quantity == order.Quantity && asset.CostBasis == order.CostBasis {
				asset_found = true
				switch asset.Type {
				case "limit_buy":
					if quote.LastPrice <= order.CostBasis {
						user.ClosePosition(asset.Type, order.Symbol, int64(order.Quantity), order.CostBasis, source)
						user.CreatePosition("long", order.Symbol, int64(order.Quantity), 0.0, source)
						source.Say("<@%s>'s limit buy has been completed.", user.UserID)
						return true
					}
					break
				case "limit_sell":
					if quote.LastPrice >= order.CostBasis {
						user.ClosePosition(order.Type, order.Symbol, int64(order.Quantity), order.CostBasis, source)
						user.ClosePosition("long", order.Symbol, int64(order.Quantity), 0.0, source)
						source.Say("<@%s>'s limit sell has been completed.", user.UserID)
						return true
					}
					break
				case "limit_cover":
					if quote.LastPrice <= order.CostBasis {
						user.ClosePosition(order.Type, order.Symbol, int64(order.Quantity), order.CostBasis, source)
						user.ClosePosition("short", order.Symbol, int64(order.Quantity), 0.0, source)
						source.Say("<@%s>'s limit cover has been completed.", user.UserID)
						return true
					}
					break
				}
			}
		}

		return !asset_found
	})
}
