package commands

import (
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
	btnSubscription := menu.InlineKeyboard[1][0]

	bot.Handle("/start", func(c telebot.Context) error {
		return showStartMenu(c)
	})

	bot.Handle(&btnWgConfigs, HandleWireguardConfigsButton)
	bot.Handle(&btnVless, HandleVlessButton)
	bot.Handle(&btnRegister, HandleRegisterCommand)
	bot.Handle(&btnHelp, HandleHelpButton)
	bot.Handle(&btnSubscription, HandleSubscription)

	bot.Handle(telebot.OnCallback, func(c telebot.Context) error {
		data := callbackData(c.Callback().Data)

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
		case data == "send_payment_request":
			return HandleSendPaymentRequest(c)
		case strings.HasPrefix(data, "choose_subscription_package|"):
			return HandleChooseSubscriptionPackage(c)
		case strings.HasPrefix(data, "submit_payment_request|"):
			return HandleSubmitPaymentRequest(c)
		case strings.HasPrefix(data, "approve_deposit|"), strings.HasPrefix(data, "deny_deposit|"):
			return HandleDepositAction(c)
		case data == "cancel_payment_and_return_to_start":
			_ = c.Respond(&telebot.CallbackResponse{})

			kb := &telebot.ReplyMarkup{}
			btnToStart := kb.Data("К началу", "to_start")
			kb.Inline(kb.Row(btnToStart))

			return c.Send("Действие отменено 👍")
		default:
			return c.Respond()
		}
	})
}
