package commands

import telebot "gopkg.in/telebot.v4"

func RegisterCommands(bot *telebot.Bot) {
	bot.Handle("/start", showStartMenu)
}
