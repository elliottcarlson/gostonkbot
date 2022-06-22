package main

type MessageStruct struct {
	SlackMessage string
	APIMessage   string
}

var (
	MessageInvalidArgument = MessageStruct{
		SlackMessage: "I'm having trouble parsing your request; expecting a %s, but something is wrong... wanna try that again?",
		APIMessage:   "",
	}

	MessageInsufficientFunds = MessageStruct{
		SlackMessage: "tester",
		APIMessage:   "tester",
	}
)
