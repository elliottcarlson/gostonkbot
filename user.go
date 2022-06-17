package main

import (
	"math"
	"strings"

	log "github.com/sirupsen/logrus"
)

var DEFAULT_WALLET_VALUE = float64(1000000)

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
			UserID:   userID,
			FullName: slackuser.Profile.RealName,
			Funds:    DEFAULT_WALLET_VALUE,
		}

		Redis.Set(userID, user)
		return user
	}
}

func (u *User) log(fields ...map[string]interface{}) *log.Entry {
	output := log.WithFields(log.Fields{
		"source":    "user.go",
		"user_id":   u.UserID,
		"user_name": u.FullName,
	})

	if fields != nil {
		output = output.WithFields(fields[0])
	}

	return output
}

func (u *User) Save() {
	u.log(map[string]interface{}{
		"funds":      u.Funds,
		"held_funds": u.HeldFunds,
	}).Info("Saving user record.")
	Redis.Set(u.UserID, u)
}

func (u *User) CreatePosition(position_type string, symbol string, quantity int64, target float64, source *Command) {
	user := u
	tradingview.GetQuote(symbol, func(quote TradingViewQuote) (shouldDelete bool) {
		user = GetUserByID(user.UserID)

		log := user.log(map[string]interface{}{
			"method":       "CreatePosition",
			"type":         position_type,
			"symbol":       symbol,
			"quantity":     quantity,
			"target_price": target,
			"last_price":   quote.LastPrice,
		})

		cost_basis := quote.LastPrice
		if quote.LivePrice != 0.00 && (quote.CurrentSession == "pre_market" || quote.CurrentSession == "post_market") {
			cost_basis = quote.LivePrice
		}
		if strings.HasPrefix(position_type, "limit_") {
			cost_basis = target
		}

		if quote.Symbol != symbol {
			log.Info("Symbol not found.")
			source.Say("<@s> I was unable to find that stock; wanna try that again?")
			return true
		}

		if position_type != "limit_sell" {
			if cost_basis*float64(quantity) > user.Funds {
				log.WithFields(map[string]interface{}{
					"cost": cost_basis * float64(quantity),
				}).Info("Insufficient funds.")
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
			log.Info("Bought shares.")
			action = "bought"
		case "short":
			log.Info("Shorted shares.")
			action = "shorted"
		case "limit_buy":
			log.Info("Created a limit buy order.")
			action = "created a limit order to buy"
			user.HeldFunds = user.HeldFunds + (cost_basis * float64(quantity))
			user.WatchLimitOrder(asset, source)
		case "limit_sell":
			log.Info("Created a limit sell order.")
			action = "created a limit order to sell"
			user.WatchLimitOrder(asset, source)
		case "limit_cover":
			log.Info("Created a limit cover order.")
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
	tradingview.GetQuote(symbol, func(quote TradingViewQuote) (shouldDelete bool) {
		user = GetUserByID(user.UserID)

		log := user.log(map[string]interface{}{
			"method":     "ClosePosition",
			"type":       position_type,
			"symbol":     symbol,
			"quantity":   quantity,
			"basis":      basis,
			"last_price": quote.LastPrice,
		})

		if quote.Symbol != symbol {
			log.Info("Symbol not found.")
			source.Say("<@s> I was unable to find that stock; wanna try that again?")
			return true
		}

		cost_basis := quote.LastPrice
		if quote.LivePrice != 0.00 && (quote.CurrentSession == "pre_market" || quote.CurrentSession == "post_market") {
			cost_basis = quote.LivePrice
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
					value := cost_basis * float64(to_sell)

					switch position_type {
					case "long":
						gains = gains + (value - proceeds)
						funds = funds + value
						log.WithFields(map[string]interface{}{
							"gains": (value - proceeds),
							"value": +value,
						}).Info("Closing long position.")
					case "short":
						gains = gains + (proceeds - value)
						funds = funds + proceeds + gains
						log.WithFields(map[string]interface{}{
							"gains": (proceeds - value),
							"value": +(proceeds + gains),
						}).Info("Closing short position.")
					case "limit_buy":
						refund := float64(to_sell) * asset.CostBasis
						user.HeldFunds = user.HeldFunds - refund
						user.Funds = user.Funds + refund
						log.WithFields(map[string]interface{}{
							"refund": refund,
						}).Info("Closing limit buy position.")
					case "limit_sell":
						log.Info("Closing limit sell position.")
					case "limit_cover":
						refund := float64(to_sell) * asset.CostBasis
						user.HeldFunds = user.HeldFunds - refund
						user.Funds = user.Funds + refund
						log.WithFields(map[string]interface{}{
							"refund": refund,
						}).Info("Closing limit cover position.")
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

		source.Say("<@%s>%s %d shares of %s at $%.2f, totalling $%.2f, netting them $%.2f. They have $%.2f funds remaining.", user.UserID, description, sold, symbol, cost_basis, funds, gains, user.Funds)
		return true
	})
}

func (u *User) WatchLimitOrder(order *Asset, source *Command) {
	user := u
	user.log(map[string]interface{}{
		"method":       "WatchLimitOrder:OnUpdate",
		"type":         order.Type,
		"symbol":       order.Symbol,
		"quantity":     order.Quantity,
		"target_price": order.CostBasis,
	}).Info("Creating a new watch limit order job.")

	tradingview.OnUpdate(order.Symbol, func(quote TradingViewQuote) (shouldDelete bool) {
		user = GetUserByID(user.UserID)

		log := user.log(map[string]interface{}{
			"method":       "WatchLimitOrder:OnUpdate",
			"type":         order.Type,
			"symbol":       order.Symbol,
			"quantity":     order.Quantity,
			"target_price": order.CostBasis,
			"last_price":   quote.LastPrice,
		})

		cost_basis := quote.LastPrice
		if quote.LivePrice != 0.00 && (quote.CurrentSession == "pre_market" || quote.CurrentSession == "post_market") {
			cost_basis = quote.LivePrice
		}

		asset_found := false
		for i := range user.Portfolio {
			asset := user.Portfolio[i]

			if asset.Type == order.Type && asset.Symbol == order.Symbol && asset.Quantity == order.Quantity && asset.CostBasis == order.CostBasis {
				asset_found = true
				switch asset.Type {
				case "limit_buy":
					if cost_basis <= order.CostBasis {
						log.Info("Limit buy has been met; closing original position, and creating long.")
						user.ClosePosition(asset.Type, order.Symbol, int64(order.Quantity), order.CostBasis, source)
						user.CreatePosition("long", order.Symbol, int64(order.Quantity), 0.0, source)
						source.Say("<@%s>'s limit buy has been completed.", user.UserID)
						return true
					}
				case "limit_sell":
					if cost_basis >= order.CostBasis {
						log.Info("Limit sell has been met; closing original position, and creating long.")
						user.ClosePosition(order.Type, order.Symbol, int64(order.Quantity), order.CostBasis, source)
						user.ClosePosition("long", order.Symbol, int64(order.Quantity), 0.0, source)
						source.Say("<@%s>'s limit sell has been completed.", user.UserID)
						return true
					}
				case "limit_cover":
					if cost_basis <= order.CostBasis {
						log.Info("Limit cover has been met; closing original position, and creating long.")
						user.ClosePosition(order.Type, order.Symbol, int64(order.Quantity), order.CostBasis, source)
						user.ClosePosition("short", order.Symbol, int64(order.Quantity), 0.0, source)
						source.Say("<@%s>'s limit cover has been completed.", user.UserID)
						return true
					}
				}
			}
		}

		log.WithFields(map[string]interface{}{
			"asset_found":    asset_found,
			"deleting_watch": !asset_found,
		}).Info("No criteria met to action on watch order.")

		return !asset_found
	})
}
