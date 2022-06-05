package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"github.com/iancoleman/strcase"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

var slackapi = slack.New(os.Getenv("SLACK_TOKEN"))
var signingSecret = os.Getenv("SLACK_SIGNING_SECRET")

func SlackEventRouter() *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/slack/events", SlackEventHandler)

	return router
}

func SlackEventHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	verifier, err := slack.NewSecretsVerifier(r.Header, signingSecret)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, err := verifier.Write(body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := verifier.Ensure(); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	event, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if event.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(body), &r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text")
		w.Write([]byte(r.Challenge))
	}

	if event.Type == slackevents.CallbackEvent {
		innerEvent := event.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			SlackMessageHandler(ev)
		}
	}
}

func SlackMessageHandler(event *slackevents.MessageEvent) {
	re := regexp.MustCompile(`^\!([a-z]+)\b\ ?(.*)?$`)
	parsed := re.FindAllStringSubmatch(event.Text, -1)

	if len(parsed) >= 1 {
		command_name := parsed[0][1]

		var args []string
		if len(parsed[0]) > 2 {
			args = strings.Split(strings.TrimSpace(parsed[0][2]), " ")
		}

		command := &Command{
			Event: event,
			User:  GetUserByID(event.User),
			Args:  args,
		}

		ref := reflect.TypeOf(command)
		if function, ok := ref.MethodByName("Command" + strcase.ToCamel(command_name)); !ok || function.Type == nil {
			fmt.Println("Unimplemented function: " + command_name)
		} else {
			function.Func.Call([]reflect.Value{
				reflect.ValueOf(command),
			})
			return
		}
	}

	// Check if the inbound message contains a $SYMBOL
	re = regexp.MustCompile(`(?:\A|\s)\$([a-zA-Z\.-]+)\b`)
	symbols := re.FindAllStringSubmatch(event.Text, -1)
	seen := map[string]bool{}
	for i := range symbols {
		if _, ok := seen[symbols[i][1]]; !ok {
			symbol := strings.ToUpper(symbols[i][1])
			seen[symbol] = true

			watchlist.GetQuote(symbol, func(quote TradingViewQuote) (shouldDelete bool) {
				if quote.Symbol != symbol {
					slackapi.PostMessage(
						event.Channel,
						slack.MsgOptionText(fmt.Sprintf("Could not find a stock under the name %s", symbol), false),
					)
				} else {
					slackapi.PostMessage(
						event.Channel,
						slack.MsgOptionText("Quote response", false),
						slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{
							UnfurlLinks: false,
							UnfurlMedia: false,
						}),
						StockQuoteBlock(quote),
					)
				}

				return true
			})
		}
	}

}
