package commands

import telebot "gopkg.in/telebot.v4"

func getStartKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}

	btnSite := kb.URL("Сайт", "https://oksana1984.ru")
	btnNews := kb.URL("Новости", "https://t.me/+DfexxpJzKiFkNzQ6")
	btnChat := kb.URL("Чат", "https://t.me/+jG8T4yBk0tg4MWNi")

	kb.Inline(
		kb.Row(btnSite),
		kb.Row(btnNews),
		kb.Row(btnChat),
	)

	return kb
}

func getStartMessage() string {
	return "👋 Добро пожаловать в <b>Oksana VPN</b>!\n\nЕсли мини-приложение недоступно, настроить VPN можно на <a href=\"https://oksana1984.ru\">сайте</a>."
}

func showStartMenu(c telebot.Context) error {
	return c.Send(getStartMessage(), &telebot.SendOptions{
		ParseMode:   telebot.ModeHTML,
		ReplyMarkup: getStartKeyboard(),
	})
}
