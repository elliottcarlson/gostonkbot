package main

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"

	//	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type Command struct {
	Event *slackevents.MessageEvent
	User  *User
	Args  []string
}

var format = message.NewPrinter(language.English)
var invalid_arg = "I'm having trouble parsing your request; expecting a %s, but something is wrong... wanna try that again?"

func (c *Command) Say(msg string, formatting ...interface{}) {
	slackapi.PostMessage(
		c.Event.Channel,
		slack.MsgOptionBlocks(
			slack.NewSectionBlock(
				slack.NewTextBlockObject(slack.MarkdownType, format.Sprintf(msg, formatting...), false, false),
				nil,
				nil,
			),
		),
	)
}

func (c *Command) GetArgAsInteger(position int) (value int64, err error) {
	if len(c.Args)-1 < position {
		return 0, fmt.Errorf("missing arguments")
	}

	if value, err = strconv.ParseInt(c.Args[position], 10, 32); err != nil {
		re := regexp.MustCompile(`^<tel:([0-9]+)\|[0-9]+>$`)
		parsed := re.FindStringSubmatch(c.Args[position])
		if len(parsed) == 2 {
			return strconv.ParseInt(parsed[1], 10, 32)
		}
	}
	return value, err
}

func (c *Command) GetArgAsString(position int) (value string, err error) {
	if len(c.Args)-1 < position {
		return "", fmt.Errorf("missing arguments")
	}

	return c.Args[position], nil
}

func (c *Command) GetArgAsStockSymbol(position int) (symbol string, err error) {
	if len(c.Args)-1 < position {
		return "", fmt.Errorf("missing arguments")
	}

	re := regexp.MustCompile(`^\$?([a-zA-Z]+)$`)
	parsed := re.FindStringSubmatch(c.Args[position])
	if len(parsed) == 2 {
		return strings.ToUpper(parsed[1]), nil
	}

	return "", fmt.Errorf("unable to parse as stock symbol")
}

func (c *Command) GetArgAsFloat(position int) (value float64, err error) {
	if len(c.Args)-1 < position {
		return 0, fmt.Errorf("missing arguments")
	}

	if value, err = strconv.ParseFloat(c.Args[position], 64); err != nil {
		re := regexp.MustCompile(`^\$?([0-9]+\.?[0-9]{1,2}?)$`)
		parsed := re.FindStringSubmatch(c.Args[position])
		if len(parsed) == 2 {
			return strconv.ParseFloat(parsed[1], 64)
		}
	}
	return value, err
}

func (c *Command) GetOptionalUserFromArg(position int) (user *User) {
	if len(c.Args)-1 < position {
		return c.User
	}
	re := regexp.MustCompile(`^<@([A-Z0-9]+)>$`)
	parsed := re.FindStringSubmatch(c.Args[position])

	if len(parsed) == 2 {
		return GetUserByID(parsed[1])
	}

	return c.User
}

/* ***********************************************************************************
 * Help - returns a listing of available commands.
 *
 * Syntax: [!help|!h]
 */
func (c *Command) CommandH() { c.CommandHelp() }
func (c *Command) CommandHelp() {
	var topic string
	var response string
	topic, _ = c.GetArgAsString(0)

	switch strings.ToLower(topic) {
	case "funds":
		response = "*!funds {@username}*\nSee your available funds. Optionally specify a target user to see their available funds. You can use `!f` as a shorthand alias to this command."
	case "portfolio":
		response = "*!portfolio {@username}*\nSee your portfolio. Optionally specify a target user to see their portfolio. You can use `!p` as a shorthand alias to this command."
	case "buy":
		response = "*!buy [quantity] [symbol]*\nPurchase the specified amount of shares in the specified stock, at the latest market price."
	case "sell":
		response = "*!sell [quantity] [symbol] {price paid}*\nSell the specified amount of shares in the the specified stock, at the latest market price. Optionally specify the price paid to make a sale using shares that were bought at that price point."
	case "short":
		response = "*!short [quantity] [symbol]*\nShort the specified amount of shares in the specified stock, at the latest market price."
	case "cover":
		response = "*!cover [quantity] [symbol] {price paid}*\nCover the specified amount of shares in the the specified stock, at the latest market price. Optionally specify the price paid to cover shares that were shorted at that price point."
	case "orders":
		response = "*!orders {@username}*\nSee your limit orders. Optionally specify a target user to see their pending limit orders. You can use `!o` as a shorthand alias to this command."
	case "limit":
		response = "*!limit [type] [quantity] [symbol] [price target]*\nCreate a limit order of the specified type (buy/sell/cover) for the specified amount of shares. When the price target is met, then your order will be executed. Limit orders are good until cancelled."
	case "cancel":
		response = "*!cancel [type] [quantity] [symbol] [price target]*\nCancel a limit order placed - arguments must match a limit order you previously created."
	case "liquidate":
		response = "*!liquidate*\nSell and cover all your shares at the current market price. Will also cancel any limit orders you have in place."
	case "bankruptcy":
		response = "*!bankruptcy*\nFile for bankruptcy and reset your stonk market account."
	case "leaderboard":
		response = "*!leaderboard*\nShow the current leaderboard of all stonk market players. You can use `!l` as a shorthand alias to this command."
	default:
		response = "Welcome to the Stonks Game - use `!help <topic>` to get more information. Available topics are: `funds`, `portfolio`, `buy`, `sell`, `short`, `cover`, `orders`, `limit`, `cancel`, `liquidate`, `bankruptcy`, `leaderboard`."
	}

	c.Say(response)
}

/* ***********************************************************************************
 * Funds - get the available funds of the initiator, or specified person.
 *
 * Syntax: [!funds|!f] [@mention:optional]
 */
func (c *Command) CommandF() { c.CommandFunds() }
func (c *Command) CommandFunds() {
	user := c.GetOptionalUserFromArg(0)

	c.Say("<@%s> has $%.2f available for investing.", user.UserID, user.Funds)
}

/* ***********************************************************************************
 * Portfolio - get the portfolio of the initiator, or specified person.
 *
 * Syntax: [!portfolio|!p] [@mention:optional]
 */
func (c *Command) CommandP() { c.CommandPortfolio() }
func (c *Command) CommandPortfolio() {
	user := c.GetOptionalUserFromArg(0)

	pronoun := "Your"
	if user.UserID != c.User.UserID {
		pronoun = "Their"
	}

	if len(user.Portfolio) == 0 {
		c.Say("<@%s>'s portfolio is empty! %s available funds are: $%.2f", user.UserID, pronoun, user.Funds)
		return
	}

	portfolio := []string{
		fmt.Sprintf("%5s | %8s | %8s | %12s | %12s | %12s | %12s", "Type", "Symbol", "Qty", "Price Paid", "Last Price", "Curr Value", "Gain"),
	}

	var gains float64
	var total float64
	var positions int

	for i := range user.Portfolio {
		asset := user.Portfolio[i]
		quote, ok := watchlist.GetCurrent(asset.Symbol)
		if !ok {
			c.Say("Unable to include your asset of %s. This might be a temporary glitch. Please try again later.", asset.Symbol)
			continue
		}

		var net float64
		switch asset.Type {
		case "long":
			net = float64(asset.Quantity) * (quote.LastPrice - asset.CostBasis)
		case "short":
			net = float64(asset.Quantity) * (asset.CostBasis - quote.LastPrice)
		default:
			continue
		}
		value := float64(asset.Quantity) * quote.LastPrice

		portfolio = append(portfolio,
			fmt.Sprintf("%5s | %8s | %8d | %12s | %12s | %12s | %12s",
				asset.Type, asset.Symbol, asset.Quantity,
				format.Sprintf("$%.4f", asset.CostBasis),
				format.Sprintf("$%.4f", quote.LastPrice),
				format.Sprintf("$%.2f", value),
				format.Sprintf("$%+.2f", net),
			),
		)
		gains = gains + net
		total = total + value
		positions = positions + 1
	}

	if positions == 0 {
		c.Say("<@%s>'s portfolio is empty! %s available funds are: $%.2f", user.UserID, pronoun, user.Funds)
		return
	}

	portfolio = append(portfolio,
		fmt.Sprintf("%44s %12s | %12s | %12s", "", "Totals:",
			format.Sprintf("$%.2f", total),
			format.Sprintf("$%+.2f", gains),
		),
	)

	c.Say("<@%s>'s portfolio:\n```%s```\nThey have $%.2f available for investing.", user.UserID, strings.Join(portfolio[:], "\n"), user.Funds)
}

/* ***********************************************************************************
 * Buy - Purchase a stock at market price
 *
 * Syntax: !buy [quantity:int] [symbol:str]
 */
func (c *Command) CommandBuy() {
	var err error
	var quantity int64
	var symbol string

	if symbol, err = c.GetArgAsStockSymbol(1); err != nil {
		c.Say(invalid_arg, "stock symbol")
		return
	}

	if quantity, err = c.GetArgAsInteger(0); err != nil {
		value, err := c.GetArgAsString(0)
		if err != nil {
			c.Say(invalid_arg, "quantity")
			return
		}

		if strings.ToLower(value) != "max" {
			c.Say(invalid_arg, "quantity")
			return
		}

		funds := c.User.Funds
		fmt.Println("???")
		watchlist.GetQuote(symbol, func(quote TradingViewQuote) (shouldDelete bool) {
			cost_basis := quote.LastPrice
			if quote.LivePrice != 0.00 && (quote.CurrentSession == "pre_market" || quote.CurrentSession == "post_market") {
				cost_basis = quote.LivePrice
			}

			quantity = int64(math.Floor(funds / cost_basis))

			fmt.Printf("Funds: %f, Cost: %f, quantity: %d\n", funds, cost_basis, quantity)

			c.User.CreatePosition("long", symbol, quantity, 0.0, c)
			return true
		})
		return
	}

	c.User.CreatePosition("long", symbol, quantity, 0.0, c)
}

/* ***********************************************************************************
 * Short - Short a stock, expecting the price to go down.
 *
 * Syntax: !short [quantity:int] [symbol:str]
 */
func (c *Command) CommandShort() {
	var err error
	var quantity int64
	var symbol string

	if quantity, err = c.GetArgAsInteger(0); err != nil {
		c.Say(invalid_arg, "quantity")
		return
	}

	if symbol, err = c.GetArgAsStockSymbol(1); err != nil {
		c.Say(invalid_arg, "stock symbol")
		return
	}

	c.User.CreatePosition("short", symbol, quantity, 0.0, c)
}

/* ***********************************************************************************
 * Sell - Sell a regularly held stock for market price. If the user is holding multiple
 *        long positions on a stock, they can specify the cost basis they bought the
 *	  stock at to sell of those.
 *
 * Syntax: !sell [quantity:int] [symbol:str] [cost_basis:float:optional]
 */
func (c *Command) CommandSell() {
	var err error
	var quantity int64
	var symbol string
	var basis float64

	if quantity, err = c.GetArgAsInteger(0); err != nil {
		c.Say(invalid_arg, "quantity")
		return
	}

	if symbol, err = c.GetArgAsStockSymbol(1); err != nil {
		c.Say(invalid_arg, "stock symbol")
		return
	}

	// Optional
	basis, _ = c.GetArgAsFloat(2)

	c.User.ClosePosition("long", symbol, quantity, basis, c)
}

/* ***********************************************************************************
 * Cover - Cover a shorted stock at market price. If the user is holding multiple
 *         shorts on a stock, they can specify the cost basis they shorted the
 * 	   stock at to cover those.
 *
 * Syntax: !cover [quantity:int] [symbol:str] [cost_basis:float:optional]
 */
func (c *Command) CommandCover() {
	var err error
	var quantity int64
	var symbol string
	var basis float64

	if quantity, err = c.GetArgAsInteger(0); err != nil {
		c.Say(invalid_arg, "quantity")
		return
	}

	if symbol, err = c.GetArgAsStockSymbol(1); err != nil {
		c.Say(invalid_arg, "stock symbol")
		return
	}

	// Optional
	basis, _ = c.GetArgAsFloat(2)

	c.User.ClosePosition("short", symbol, quantity, basis, c)
}

/* ***********************************************************************************
 * Orders - get the pending order book of the initiator, or specified person.
 *
 * Syntax: [!orders|!o] [@mention:optional]
 */
func (c *Command) CommandO() { c.CommandOrders() }
func (c *Command) CommandOrders() {
	user := c.GetOptionalUserFromArg(0)

	pronoun := "Your"
	if user.UserID != c.User.UserID {
		pronoun = "Their"
	}

	if len(user.Portfolio) == 0 {
		c.Say("<@%s> doesn't have any pending orders! %s available funds are: $%.2f", user.UserID, pronoun, user.Funds)
		return
	}

	portfolio := []string{
		fmt.Sprintf("%11s | %8s | %8s | %14s | %12s", "Limit Type", "Symbol", "Qty", "Target Price", "Last Price"),
	}

	var positions int
	for i := range user.Portfolio {
		asset := user.Portfolio[i]

		switch asset.Type {
		case "limit_buy":
			asset.Type = "buy"
		case "limit_sell":
			asset.Type = "sell"
		case "limit_cover":
			asset.Type = "cover"
		default:
			continue
		}

		quote, ok := watchlist.GetCurrent(asset.Symbol)
		if !ok {
			c.Say("Unable to include your asset of %s. This might be a temporary glitch. Please try again later.", asset.Symbol)
			continue
		}

		portfolio = append(portfolio,
			fmt.Sprintf("%11s | %8s | %8d | %14s | %12s",
				asset.Type, asset.Symbol, asset.Quantity,
				format.Sprintf("$%.4f", asset.CostBasis),
				format.Sprintf("$%.4f", quote.LastPrice),
			),
		)
		positions = positions + 1
	}

	if positions == 0 {
		c.Say("<@%s> doesn't have any pending orders! %s available funds are: $%.2f", user.UserID, pronoun, user.Funds)
		return
	}

	c.Say("<@%s>'s pending orders:\n```%s```\nThey have $%.2f being held to cover buy orders, and $%.2f available for investing.", user.UserID, strings.Join(portfolio[:], "\n"), user.HeldFunds, user.Funds)
}

/* ***********************************************************************************
 * Limit - Create a limit buy or sell order. If a limit buy is placed, when the stock
 *         hits the target price or goes below, then a buy order will be executed; if
 *         a limit sell is placed, when the stock hits the target price or goes above,
 *         then a sell order will be executed. When placing a limit order, funds to
 *         cover the order at the target price will be held until the order is
 *         finalized, or cancelled. Limit orders do not expire automatically.
 *
 * Syntax: !limit [type:"buy"|"sell"] [quantity:int] [symbol:str] [target:float]
 */
func (c *Command) CommandLimit() {
	var err error
	var limit string
	var quantity int64
	var symbol string
	var target float64

	if limit, err = c.GetArgAsString(0); err != nil || limit == "" {
		c.Say(invalid_arg, "limit type")
		return
	}
	switch strings.ToLower(limit) {
	case "buy":
		limit = "limit_buy"
	case "sell":
		limit = "limit_sell"
	case "cover":
		limit = "limit_cover"
	default:
		c.Say("Unknown limit type specified. Valid types are `buy` or `sell`.")
		return
	}

	if quantity, err = c.GetArgAsInteger(1); err != nil {
		c.Say(invalid_arg, "quantity")
		return
	}

	if symbol, err = c.GetArgAsStockSymbol(2); err != nil {
		c.Say(invalid_arg, "stock symbol")
		return
	}

	if target, err = c.GetArgAsFloat(3); err != nil {
		c.Say(invalid_arg, "limit price")
		return
	}

	source := c
	user := source.User
	user.CreatePosition(limit, symbol, quantity, target, c)
}

/* ***********************************************************************************
 * Cancel - Cancel a pending limit order.
 *
 * Syntax: !limit [type:"buy"|"sell"] [quantity:int] [symbol:str] [target:float]
 */
func (c *Command) CommandCancel() {
	var err error
	var limit string
	var quantity int64
	var symbol string
	var target float64

	if limit, err = c.GetArgAsString(0); err != nil || limit == "" {
		c.Say(invalid_arg, "limit type")
		return
	}
	switch strings.ToLower(limit) {
	case "buy":
		limit = "limit_buy"
	case "sell":
		limit = "limit_sell"
	case "cover":
		limit = "limit_cover"
	default:
		c.Say("Unknown limit type specified. Valid types are `buy` or `sell`.")
		return
	}

	if quantity, err = c.GetArgAsInteger(1); err != nil {
		c.Say(invalid_arg, "quantity")
		return
	}

	if symbol, err = c.GetArgAsStockSymbol(2); err != nil {
		c.Say(invalid_arg, "stock symbol")
		return
	}

	if target, err = c.GetArgAsFloat(3); err != nil {
		c.Say(invalid_arg, "limit price")
		return
	}

	portfolio := c.User.Portfolio
	for i := range portfolio {
		asset := portfolio[i]
		if asset.Type == limit && asset.Symbol == symbol && asset.Quantity == int(quantity) && asset.CostBasis == target {
			c.Say("<@%s>, your limit order for %s has been cancelled.", c.User.UserID, asset.Symbol)
			c.User.ClosePosition(asset.Type, asset.Symbol, int64(asset.Quantity), asset.CostBasis, c)
			return
		}
	}

	c.Say("I was unable to find that order; make sure you entered the right information.")
}

/* ***********************************************************************************
 * Liquidate - liquidate all your current positions at the current market price.
 *
 * Syntax: !liquidate
 */
func (c *Command) CommandLiquidate() {
	portfolio := c.User.Portfolio
	for i := range portfolio {
		asset := portfolio[i]
		c.Say("<@%s>, your limit order for %d of %s at $%.2f has been cancelled.", c.User.UserID, asset.Quantity, asset.Symbol, asset.CostBasis)
		c.User.ClosePosition(asset.Type, asset.Symbol, int64(asset.Quantity), asset.CostBasis, c)
	}
}

/* ***********************************************************************************
 * Bankruptcy - declare bankruptcy and fully resets your account to default values.
 *
 * Syntax: !bankruptcy
 */
func (c *Command) CommandBankruptcy() {
	user := c.User
	t := time.Now()

	Redis.Delete(user.UserID)

	c.Say("<!channel> Notice is hereby given, that on the %s day of %s, A. D. %s, <@%s> was duly adjudicated bankrupt. If they owed you anything, tough shit.\n", humanize.Ordinal(t.Day()), t.Month(), strconv.Itoa(t.Year()), user.UserID)
}

/* ***********************************************************************************
 * Leaderboard - Show the leaderboard based on total funds and portfolio value
 *
 * Syntax: !leaderboard
 */
func (c *Command) CommandL() { c.CommandLeaderboard() }
func (c *Command) CommandLeaderboard() {
	users := Redis.GetAllUsers()

	type LeaderBoardEntry struct {
		UserName string
		NetWorth float64
	}

	var leaderboard []*LeaderBoardEntry

	for i := range users {
		user := users[i]
		networth := 0.0
		for j := range user.Portfolio {
			asset := user.Portfolio[j]
			if quote, ok := watchlist.GetCurrent(asset.Symbol); ok {
				networth = networth + (float64(asset.Quantity) * quote.LastPrice)
			}
		}

		leaderboard = append(leaderboard, &LeaderBoardEntry{
			UserName: user.FullName,
			NetWorth: networth + user.Funds + user.HeldFunds,
		})
	}

	sort.Slice(leaderboard[:], func(i, j int) bool {
		return leaderboard[i].NetWorth > leaderboard[j].NetWorth
	})

	composed := []string{
		fmt.Sprintf("%2s | %34s | %18s", "#", "Bag Holder", "Net Worth"),
	}

	for i := range leaderboard {
		composed = append(composed, fmt.Sprintf("%2d | %34s | %18s", i, leaderboard[i].UserName, format.Sprintf("$%.2f", leaderboard[i].NetWorth)))
	}

	c.Say("The current leaderboard:\n```%s```", strings.Join(composed[:], "\n"))
}
