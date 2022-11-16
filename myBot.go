package main

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/emersion/go-imap/client"

	"github.com/emersion/go-smtp"
)

func main() {

	config, err := LoadConfig(".")
	if err != nil {
		log.Fatal("cannot load config:", err)
	}

	bot, err := tgbotapi.NewBotAPI(config.APIToken)
	if err != nil {
		panic(err)
	}

	c, err := client.DialTLS("mail.example.org:993", nil)

	if err != nil {
		log.Fatal(err)
	}

	// Don't forget to logout
	defer c.Logout()

	bot.Debug = true
	// Create a new UpdateConfig struct with an offset of 0. Offsets are used
	// to make sure Telegram knows we've handled previous values and we don't
	// need them repeated.
	updateConfig := tgbotapi.NewUpdate(0)

	// Tell Telegram we should wait up to 30 seconds on each request for an
	// update. This way we can get information just as quickly as making many
	// frequent requests without having to send nearly as many.
	updateConfig.Timeout = 30

	// Start polling Telegram for updates.
	updates := bot.GetUpdatesChan(updateConfig)

	// Let's go through each update that we're getting from Telegram.
	for update := range updates {
		// Telegram can send many types of updates depending on what your Bot
		// is up to. We only want to look at messages for now, so we can
		// discard any other updates.
		if update.Message == nil {
			continue
		}
		if update.Message.Chat.ID != config.ChatId {
			continue
		}

		// Now that we know we've gotten a new message, we can construct a
		// reply! We'll take the Chat ID and Text from the incoming message
		// and use it to create a new message.
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "dasd")
		// We'll also say that this message is a reply to the previous message.
		// For any other specifications than Chat ID or Text, you'll need to
		// set fields on the `MessageConfig`.
		msg.ReplyToMessageID = update.Message.MessageID

		switch update.Message.Text {
		case "open":
			msg.ReplyMarkup = "1"
		case "close":
			msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		}

		// Okay, we're sending our message off! We don't care about the message
		// we just sent, so we'll discard it.
		if _, err := bot.Send(msg); err != nil {
			// Note that panics are a bad way to handle errors. Telegram can
			// have service outages or network errors, you should retry sending
			// messages or more gracefully handle failures.
			panic(err)
		}
	}
	// Setup an unencrypted connection to a local mail server.
	cl, err := smtp.Dial("localhost:25")
	if err != nil {
		return
	}
	defer cl.Close()
}
