package commands

import (
	"bytes"
	"fmt"
	"oksana-vpn-telegram-bot/pkg/api"
	"oksana-vpn-telegram-bot/pkg/utils"
	"os"
	"path/filepath"
	"strings"

	telebot "gopkg.in/telebot.v4"
)

func getMainMenu() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	btnWgConfigs := menu.Data("WireGuard", "wireguard_menu_configs")
	btnVless := menu.Data("VLESS", "vless_menu")
	btnBalance := menu.Data("Баланс", "menu_balance")
	btnHelp := menu.Data("Помощь", "menu_help")

	menu.Inline(
		menu.Row(btnWgConfigs, btnVless),
		menu.Row(btnBalance, btnHelp),
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
	return "Привет! Выбери команду:" + getTopUpReminder(status)
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

func getHelpData() (*telebot.ReplyMarkup, string) {
	kb := &telebot.ReplyMarkup{}

	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(kb.Row(btnToStart))

	help := strings.ReplaceAll(`
Впн будет через WireGuard, поэтому качайте на пк и/или телефон

Настройка для пк/телефона:
1. Скачиваете конфиг через кнопки в боте
2. Нажимаете плюсик
3. Выбираете загрузку файл
4. Жмете подключиться

Настройка для телефона: такая же, за исключением, что можно через QR код

Один конфиг можно ДОБАВИТЬ на оба устройства, но ИСПОЛЬЗОВАТЬ *одновременно* их нельзя. Если подключиться к VPN с 2 и более устройств, используя один конфиг, то работать не будет. 

*Одновременно, 1 конфиг = 1 устройство*

———

Качать отсюда: https://www.wireguard.com/install/

Там на все устройства есть ссылки

Для тех, у кого не работает, версия с усиленной маскировкой - AmneziaWG в Play Market или по ссылке https://docs.amnezia.org/ru/documentation/amnezia-wg/ (внизу страницы)
`, "\\'", "`")

	help = utils.EscapeMarkdownV2(help)

	return kb, help
}

func sendHelpPhoto(c telebot.Context) error {
	kb, message := getHelpData()

	wd, _ := os.Getwd()
	photoPath := filepath.Join(wd, "internal", "images", "help.jpg")
	data, err := os.ReadFile(photoPath)
	if err != nil {
		return c.Send("Cannot read image file: " + err.Error())
	}

	photo := &telebot.Photo{
		File: telebot.File{
			FileReader: bytes.NewReader(data),
		},
		Caption: message,
	}

	return c.Send(photo, &telebot.SendOptions{
		ParseMode:   telebot.ModeMarkdownV2,
		ReplyMarkup: kb,
	})
}

func HandleHelpCommand(c telebot.Context) error {
	return sendHelpPhoto(c)
}

func HandleHelpButton(c telebot.Context) error {
	return sendHelpPhoto(c)
}
