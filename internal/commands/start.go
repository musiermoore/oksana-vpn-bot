package commands

import (
	_ "embed"
	"fmt"
	"oksana-vpn-telegram-bot/pkg/api"
	"strings"

	telebot "gopkg.in/telebot.v4"
)

//go:embed help_wg.txt
var helpWGText string

//go:embed help_vless.txt
var helpVLESSText string

func getMainMenu() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	btnWgConfigs := menu.Data("WireGuard", "wireguard_menu_configs")
	btnVless := menu.Data("VLESS", "vless_menu")
	btnSubscription := menu.Data("Подписка", "menu_subscription")
	btnHelp := menu.Data("Помощь", "menu_help")

	menu.Inline(
		menu.Row(btnWgConfigs, btnVless),
		menu.Row(btnSubscription, btnHelp),
	)

	return menu
}

func getGuestMenu() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	btnRegister := menu.Data("Регистрация", "menu_register")
	btnHelp := menu.Data("Помощь", "menu_help")

	menu.Inline(menu.Row(btnRegister, btnHelp))

	return menu
}

func getTopUpReminder(status api.RegistrationStatus) string {
	if !status.Registered || status.HasMoneyForNextSubscriptionMonth {
		return ""
	}

	return "\n\nПополнение может понадобиться уже скоро. На следующий месяц баланса сейчас не хватает."
}

func getSubscriptionDetails(status api.RegistrationStatus) string {
	details := ""

	if status.ActiveSubscriptionEndDate != nil && strings.TrimSpace(*status.ActiveSubscriptionEndDate) != "" {
		details += fmt.Sprintf("\nПодписка активна до: %s", *status.ActiveSubscriptionEndDate)
	}

	if !status.HasMoneyForNextSubscriptionMonth {
		details += "\n\nНапоминание: баланса сейчас не хватает на следующий месяц подписки."
	}

	return details
}

func getStartMessage(status api.RegistrationStatus) string {
	return "Привет! Я помогу с подпиской и настройкой VPN.\n\nВыбери раздел:" + getTopUpReminder(status)
}

func ensureRegistered(c telebot.Context) (api.RegistrationStatus, error) {
	client := api.NewClient(c)
	status, err := client.GetRegistrationStatus()

	if err == nil && status.Registered {
		return status, nil
	}

	if err != nil && !api.IsMissingUserError(404, err.Error()) {
		return api.RegistrationStatus{}, err
	}

	if err := client.RegisterUser(); err != nil {
		return api.RegistrationStatus{}, err
	}

	status, err = client.GetRegistrationStatus()
	if err == nil {
		status.Registered = true
		return status, nil
	}

	return api.RegistrationStatus{Registered: true}, nil
}

func showStartMenu(c telebot.Context) error {
	status, err := ensureRegistered(c)
	if err != nil {
		return c.Send("Не получилось начать работу. Попробуй чуть позже.")
	}

	return c.Send(getStartMessage(status), getMainMenu())
}

func HandleRegisterCommand(c telebot.Context) error {
	client := api.NewClient(c)
	status, err := client.GetRegistrationStatus()
	if err != nil {
		return c.Send("Не получилось проверить статус регистрации. Попробуй чуть позже.")
	}

	if status.Registered {
		return c.Send("Ты уже зарегистрирован. Можно пользоваться ботом.", getMainMenu())
	}

	if err := client.RegisterUser(); err != nil {
		return c.Send("Не получилось завершить регистрацию. Попробуй чуть позже.")
	}

	return showStartMenu(c)
}

func getHelpMenuKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}

	btnWG := kb.Data("WG", "help|wg")
	btnVLESS := kb.Data("VLESS", "help|vless")
	btnClients := kb.Data("Клиенты", "help|clients")
	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(
		kb.Row(btnWG, btnVLESS),
		kb.Row(btnClients),
		kb.Row(btnToStart),
	)

	return kb
}

func getHelpWGDetailsKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}

	btnClients := kb.Data("WG клиенты", "help|clients|wg")
	btnBack := kb.Data("Назад", "help|menu")
	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(
		kb.Row(btnClients),
		kb.Row(btnBack, btnToStart),
	)

	return kb
}

func getHelpVLESSDetailsKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}

	btnClients := kb.Data("VLESS клиенты", "help|clients|vless")
	btnBack := kb.Data("Назад", "help|menu")
	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(
		kb.Row(btnClients),
		kb.Row(btnBack, btnToStart),
	)

	return kb
}

func getHelpClientsMenuKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}

	btnWGClients := kb.Data("WG клиенты", "help|clients|wg")
	btnVLESSClients := kb.Data("VLESS клиенты", "help|clients|vless")
	btnBack := kb.Data("Назад", "help|menu")
	btnToStart := kb.Data("К началу", "to_start")

	kb.Inline(
		kb.Row(btnWGClients),
		kb.Row(btnVLESSClients),
		kb.Row(btnBack, btnToStart),
	)

	return kb
}

func getHelpWGClientsKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}

	btnAmneziaWGIOS := kb.URL("Amnezia iOS", "https://apps.apple.com/us/app/amneziavpn/id1600529900")
	btnAmneziaWGAndroid := kb.URL("Amnezia Android", "https://play.google.com/store/apps/details?id=org.amnezia.awg")
	btnAmneziaWGPC := kb.URL("Сайт Amnezia", "https://amnezia.org/ru/downloads")
	btnWireGuardIOS := kb.URL("WireGuard iOS", "https://apps.apple.com/us/app/wireguard/id1441195209")
	btnWireGuardAndroid := kb.URL("WireGuard Android", "https://play.google.com/store/apps/details?id=com.wireguard.android&hl=ru")
	btnWireGuardSite := kb.URL("Сайт WireGuard", "https://www.wireguard.com/")
	btnBack := kb.Data("Назад", "help|clients")
	btnToStart := kb.Data("К началу", "to_start")

	kb.Inline(
		kb.Row(btnAmneziaWGIOS, btnAmneziaWGAndroid),
		kb.Row(btnAmneziaWGPC),
		kb.Row(btnWireGuardIOS, btnWireGuardAndroid),
		kb.Row(btnWireGuardSite),
		kb.Row(btnBack, btnToStart),
	)

	return kb
}

func getHelpVLESSClientsKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}

	btnV2RayTunIOS := kb.URL("v2raytun iOS", "https://apps.apple.com/us/app/v2raytun/id6476628951")
	btnV2RayTunAndroid := kb.URL("v2raytun Android", "https://play.google.com/store/apps/details?id=com.v2raytun.android")
	btnV2RayTunSite := kb.URL("Сайт v2raytun", "https://v2raytun.com/#download")
	btnHappAndroid := kb.URL("Happ Android", "https://play.google.com/store/apps/details?id=com.happproxy")
	btnHappIOS := kb.URL("Happ iOS", "https://apps.apple.com/us/app/happ-proxy-utility/id6504287215")
	btnHappSite := kb.URL("Сайт Happ", "https://www.happ.su/main/ru")
	btnBack := kb.Data("Назад", "help|clients")
	btnToStart := kb.Data("К началу", "to_start")

	kb.Inline(
		kb.Row(btnV2RayTunIOS, btnV2RayTunAndroid),
		kb.Row(btnV2RayTunSite),
		kb.Row(btnHappAndroid, btnHappIOS),
		kb.Row(btnHappSite),
		kb.Row(btnBack, btnToStart),
	)

	return kb
}

func getHelpWGMessage() string {
	return strings.TrimSpace(helpWGText)
}

func getHelpVLESSMessage() string {
	return strings.TrimSpace(helpVLESSText)
}

func sendHelpMenu(c telebot.Context) error {
	return c.Send("Выберите раздел помощи:", getHelpMenuKeyboard())
}

func HandleHelpMenu(c telebot.Context) error {
	return sendHelpMenu(c)
}

func HandleHelpWG(c telebot.Context) error {
	return c.Send(getHelpWGMessage(), getHelpWGDetailsKeyboard())
}

func HandleHelpVLESS(c telebot.Context) error {
	return c.Send(getHelpVLESSMessage(), getHelpVLESSDetailsKeyboard())
}

func HandleHelpClients(c telebot.Context) error {
	return c.Send("Выберите тип клиента:", getHelpClientsMenuKeyboard())
}

func HandleHelpWGClients(c telebot.Context) error {
	return c.Send("WG клиенты. Если не нашли свою платформу, откройте официальный сайт ниже.", getHelpWGClientsKeyboard())
}

func HandleHelpVLESSClients(c telebot.Context) error {
	return c.Send("VLESS клиенты. Если не нашли свою платформу, откройте официальный сайт ниже.", getHelpVLESSClientsKeyboard())
}

func HandleHelpCommand(c telebot.Context) error {
	return sendHelpMenu(c)
}

func HandleHelpButton(c telebot.Context) error {
	return sendHelpMenu(c)
}
