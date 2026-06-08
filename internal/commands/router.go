package commands

import (
	"fmt"
	"strconv"
	"strings"

	telebot "gopkg.in/telebot.v4"
)

func RegisterCommands(bot *telebot.Bot) {
	menu := getMainMenu()
	guestMenu := getGuestMenu()

	btnWgConfigs := menu.InlineKeyboard[0][0]
	btnVless := menu.InlineKeyboard[0][1]
	btnRegister := guestMenu.InlineKeyboard[0][0]
	btnHelp := guestMenu.InlineKeyboard[0][1]
	btnBalance := menu.InlineKeyboard[1][0]

	bot.Handle("/start", func(c telebot.Context) error {
		return showStartMenu(c)
	})

	bot.Handle(&btnWgConfigs, HandleWireguardConfigsButton)
	bot.Handle(&btnVless, HandleVlessButton)
	bot.Handle(&btnRegister, HandleRegisterCommand)
	bot.Handle(&btnHelp, HandleHelpButton)
	bot.Handle(&btnBalance, HandleBalance)

	bot.Handle(telebot.OnCallback, func(c telebot.Context) error {
		data := strings.TrimSpace(c.Callback().Data)

		switch {
		case strings.HasPrefix(data, "action_config_"):
			return HandleActionConfig(c)
		case strings.HasPrefix(data, "config|"):
			return HandleChoosingConfig(c)
		case data == "to_start":
			return showStartMenu(c)
		case data == "help|menu":
			return HandleHelpMenu(c)
		case data == "help|wg":
			return HandleHelpWG(c)
		case data == "help|vless":
			return HandleHelpVLESS(c)
		case data == "help|clients":
			return HandleHelpClients(c)
		case data == "help|clients|wg":
			return HandleHelpWGClients(c)
		case data == "help|clients|vless":
			return HandleHelpVLESSClients(c)
		case data == "to_wireguard_configs":
			return HandleWireguardConfigsButton(c)
		case data == "to_vless":
			return HandleVlessButton(c)
		case data == "vless|menu":
			return HandleVlessCommand(c)
		case data == "vless|configs":
			return HandleVlessConfigsButton(c)
		case data == "vless|link":
			return HandleVlessLinkAction(c)
		case data == "vless|qr":
			return HandleVlessQrAction(c)
		case strings.HasPrefix(data, "send_payment_request"):
			return HandleSendPaymentRequest(c)
		case strings.HasPrefix(data, "submit_payment_request|"):
			return HandleSubmitPaymentRequest(c)
		case strings.HasPrefix(data, "approve_deposit|"), strings.HasPrefix(data, "deny_deposit|"):
			return HandleDepositAction(c)
		case data == "cancel_payment_and_return_to_start":
			waitingForAmount[c.Sender().ID] = false
			_ = c.Respond(&telebot.CallbackResponse{})

			kb := &telebot.ReplyMarkup{}
			btnToStart := kb.Data("К началу", "to_start")
			kb.Inline(kb.Row(btnToStart))

			return c.Send("Действие отменено 👍")
		default:
			return c.Respond()
		}
	})

	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		userID := c.Sender().ID
		text := c.Message().Text

		if !waitingForAmount[userID] {
			return c.Send("Используй /start и кнопки в меню.")
		}

		kb := &telebot.ReplyMarkup{}
		btnCancel := kb.Data("Отменить", "cancel_payment_and_return_to_start")

		amount, err := strconv.ParseFloat(text, 64)
		if err != nil || amount <= 0 {
			waitingForAmount[userID] = true
			kb.Inline(kb.Row(btnCancel))
			return c.Send("Нужно ввести число больше 0.", kb)
		}

		waitingForAmount[userID] = false

		btnAdd := kb.Data("Добавить", fmt.Sprintf("submit_payment_request|%f", amount))
		kb.Inline(kb.Row(btnAdd, btnCancel))

		return c.Send(fmt.Sprintf("✅ Отправить запрос на пополение %.2f руб?", amount), kb)
	})
}
