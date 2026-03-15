package commands

import (
	"bytes"
	"fmt"
	"oksana-vpn-telegram-bot/pkg/api"
	"oksana-vpn-telegram-bot/pkg/utils"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	telebot "gopkg.in/telebot.v4"
)

var waitingForAmount = make(map[int64]bool)

func RegisterCommands(bot *telebot.Bot) {
	menu := &telebot.ReplyMarkup{}

	btnWgConfigs := menu.Data("WireGuard", "wireguard_menu_configs")
	btnVlessConfigs := menu.Data("Vless", "vless_menu_configs")
	btnBalance := menu.Data("Баланс", "menu_balance")
	btnHelp := menu.Data("Помощь", "menu_help")
	menu.Inline(
		menu.Row(btnWgConfigs, btnVlessConfigs, btnBalance, btnHelp),
	)

	bot.Handle("/start", func(c telebot.Context) error {
		return c.Send("Привет! Выбери команду:", menu)
	})

	bot.Handle("/help", HandleHelpCommand)
	bot.Handle("/wg_configs", HandleWireguardConfigsCommand)
	bot.Handle("/vless_configs", HandleVlessConfigsCommand)
	bot.Handle("/balance", HandleBalance)
	bot.Handle("/payment_request", HandleSendPaymentRequest)

	// Handle main menu buttons
	bot.Handle(&btnWgConfigs, HandleWireguardConfigsButton)
	bot.Handle(&btnVlessConfigs, HandleVlessConfigsButton)
	bot.Handle(&btnHelp, HandleHelpButton)
	bot.Handle(&btnBalance, HandleBalance)

	// Handle config buttons
	bot.Handle(telebot.OnCallback, func(c telebot.Context) error {
		data := strings.TrimSpace(c.Callback().Data)

		if strings.HasPrefix(data, "action_config_") {
			return HandleActionConfig(c)
		} else if strings.HasPrefix(data, "config|") {
			return HandleChoosingConfig(c)
		} else if data == "to_start" {
			return c.Send("Выбери команду:", menu)
		} else if data == "to_wireguard_configs" {
			return HandleWireguardConfigsButton(c)
		} else if data == "to_vless_configs" {
			return HandleVlessConfigsButton(c)
		} else if strings.HasPrefix(data, "send_payment_request") {
			return HandleSendPaymentRequest(c)
		} else if strings.HasPrefix(data, "submit_payment_request|") {
			return HandleSubmitPaymentRequest(c)
		} else if data == "cancel_payment_and_return_to_start" {
			userId := c.Sender().ID
			waitingForAmount[userId] = false

			_ = c.Respond(&telebot.CallbackResponse{})

			kb := &telebot.ReplyMarkup{}
			btnToStart := kb.Data("К началу", "to_start")
			kb.Inline(kb.Row(btnToStart))

			return c.Send("Действие отменено 👍")
		}

		return c.Respond()
	})

	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		userId := c.Sender().ID
		text := c.Message().Text

		// If waiting for amount
		if waitingForAmount[userId] {
			kb := &telebot.ReplyMarkup{}
			btnCancel := kb.Data("Отменить", "cancel_payment_and_return_to_start")

			amount, err := strconv.ParseFloat(text, 64)
			if err != nil || amount <= 0 {
				waitingForAmount[userId] = true

				kb.Inline(kb.Row(btnCancel))

				return c.Send("Нужно ввести число больше 0.", kb)
			}

			waitingForAmount[userId] = false

			btnAdd := kb.Data("Добавить", fmt.Sprintf("submit_payment_request|%f", amount))
			kb.Inline(kb.Row(btnAdd, btnCancel))

			// Handle the amount
			return c.Send(fmt.Sprintf("✅ Отправить запрос на пополение %.2f руб?", amount), kb)
		}

		// If not waiting — handle normal text
		return c.Send("Неизвестная команда. Используй /start")
	})
}

func getConfigsKeyboard(c telebot.Context, configType string) (*telebot.ReplyMarkup, *api.ConfigResponse, error) {
	client := api.NewClient(c)

	response, err := client.GetConfigs(configType)

	kb := &telebot.ReplyMarkup{}
	inline := []telebot.Row{}

	if err == nil {
		for _, config := range response.Configs {
			row := kb.Data(config.Name, "config|"+configType+"|"+strconv.Itoa(int(config.ID))+"|"+config.Name)

			inline = append(
				inline,
				kb.Row(row),
			)
		}
	} else if response.Type == "debt" {
		btnBalance := kb.Data("Баланс", "menu_balance")
		inline = append(inline, kb.Row(btnBalance))
	}

	btnToStart := kb.Data("К началу", "to_start")

	inline = append(
		inline,
		kb.Row(btnToStart),
	)
	kb.Inline(
		inline...,
	)

	return kb, &response, err
}

func HandleWireguardConfigsCommand(c telebot.Context) error {
	kb, response, err := getConfigsKeyboard(c, "wireguard")

	message := getConfigMessage(response, err, "Выбери конфиг:")

	return c.Send(message, kb)
}

func HandleWireguardConfigsButton(c telebot.Context) error {
	kb, response, err := getConfigsKeyboard(c, "wireguard")

	message := getConfigMessage(response, err, "Выбери конфиг:")

	return c.Send(message, kb)
}

func HandleVlessConfigsCommand(c telebot.Context) error {
	kb, response, err := getConfigsKeyboard(c, "vless")

	message := getConfigMessage(response, err, "Выбери конфиг:")

	return c.Send(message, kb)
}

func HandleVlessConfigsButton(c telebot.Context) error {
	kb, response, err := getConfigsKeyboard(c, "vless")

	message := getConfigMessage(response, err, "Выбери конфиг:")

	return c.Send(message, kb)
}

func getConfigMessage(response *api.ConfigResponse, err error, message string) string {
	if err != nil && response.Message != "" {
		message = response.Message
	} else if len(response.Configs) == 0 {
		message = "Конфиги не найдены"
	}

	return message
}

func getHelpData() (*telebot.ReplyMarkup, string) {
	kb := &telebot.ReplyMarkup{}

	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(kb.Row(btnToStart))

	help := strings.ReplaceAll(`
Впн будет через WireGuard, поэтому качайте на пк и/или телефон

Настройка для пк/телефона:
1. Скачиваете конфиг (Команда /configs)
2. Нажимаете плюсик
3. Выбираете загрузку файл
4. Жмете подключиться

Настройка для телефона: такая же, за исключением, что можно через QR код

Один конфиг можно ДОБАВИТЬ на оба устройства, но ИСПОЛЬЗОВАТЬ *одновременно* их нельзя. Если подключиться к VPN с 2 и более устройств, используя один конфиг, то работать не будет. 

*Одновременно, 1 конфиг = 1 устройство*

———

Качать отсюда: https://www.wireguard.com/install/

Там на все устройства есть ссылки
`, "\\'", "`")

	help = utils.EscapeMarkdownV2(help)

	return kb, help
}

func HandleHelpCommand(c telebot.Context) error {
	kb, message := getHelpData()

	wd, _ := os.Getwd()
	photoPath := filepath.Join(wd, "internal", "images", "help.jpg")
	data, err := os.ReadFile(photoPath)
	if err != nil {
		return c.Send("Cannot read image file: " + err.Error())
	}

	photo := &telebot.Photo{
		File: telebot.File{
			FileReader: bytes.NewReader(data), // file content in memory
		},
		Caption: message,
	}

	return c.Send(photo, &telebot.SendOptions{
		ParseMode:   telebot.ModeMarkdownV2,
		ReplyMarkup: kb,
	})
}

func HandleHelpButton(c telebot.Context) error {
	kb, message := getHelpData()

	wd, _ := os.Getwd()
	photoPath := filepath.Join(wd, "internal", "images", "help.jpg")
	data, err := os.ReadFile(photoPath)
	if err != nil {
		return c.Send("Cannot read image file: " + err.Error())
	}

	photo := &telebot.Photo{
		File: telebot.File{
			FileReader: bytes.NewReader(data), // file content in memory
		},
		Caption: message,
	}

	return c.Send(photo, &telebot.SendOptions{
		ParseMode:   telebot.ModeMarkdownV2,
		ReplyMarkup: kb,
	})
}

func HandleChoosingConfig(c telebot.Context) error {
	config := strings.TrimSpace(c.Callback().Data)
	parts := strings.Split(config, "|")

	configType := parts[1]
	configName := parts[3]

	kb := &telebot.ReplyMarkup{}

	btnQR := kb.Data("QR Code", "action_config_qr|"+config)

	// Show different buttons based on config type
	var actionButtons []telebot.Btn
	if configType == "vless" {
		btnLink := kb.Data("Получить ссылку", "action_config_link|"+config)
		actionButtons = []telebot.Btn{btnQR, btnLink}
	} else {
		btnDownload := kb.Data("Файл", "action_config_file|"+config)
		actionButtons = []telebot.Btn{btnQR, btnDownload}
	}

	btnWireguardConfigs := kb.Data("WireGuard Конфиги", "to_wireguard_configs")
	btnVlessConfigs := kb.Data("Vless Конфиги", "to_vless_configs")

	kb.Inline(
		kb.Row(actionButtons...),
		kb.Row(btnWireguardConfigs),
		kb.Row(btnVlessConfigs),
	)

	return c.Send("Выбери действие для конфига "+configName, kb)
}

func prepareConfigData(c telebot.Context) string {
	data := strings.TrimSpace(c.Callback().Data)
	return strings.Replace(data, "config|", "", 1)
}

func getConfigType(c telebot.Context) string {
	data := prepareConfigData(c)
	parts := strings.Split(data, "|")
	// After removing "config|", action data looks like: "action_config_qr|wireguard|123|name"
	// So configType is at index 1
	if strings.HasPrefix(data, "action_config_") {
		return parts[1]
	}
	// For non-action data: "wireguard|123|name"
	return parts[0]
}

func getConfigName(c telebot.Context) string {
	data := prepareConfigData(c)
	parts := strings.Split(data, "|")
	// After removing "config|", action data looks like: "action_config_qr|wireguard|123|name"
	// So configName is at index 3
	if strings.HasPrefix(data, "action_config_") {
		return parts[3]
	}
	// For non-action data: "wireguard|123|name"
	return parts[2]
}

func getActionConfigKeyboard(configType string) *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}

	backAction := "to_wireguard_configs"
	if configType == "vless" {
		backAction = "to_vless_configs"
	}

	btnPrev := kb.Data("Конфиги", backAction)
	btnToStart := kb.Data("К началу", "to_start")

	kb.Inline(kb.Row(btnPrev, btnToStart))

	return kb
}

func HandleActionConfig(c telebot.Context) error {
	data := prepareConfigData(c)

	configType := getConfigType(c)
	configName := getConfigName(c)
	kb := getActionConfigKeyboard(configType)

	if strings.HasPrefix(data, "action_config_qr|") {
		return HandleQrCodeConfig(c)
	} else if strings.HasPrefix(data, "action_config_file|") {
		return HandleDownloadConfig(c)
	} else if strings.HasPrefix(data, "action_config_link|") {
		return HandleGetLinkConfig(c)
	} else if strings.HasPrefix(data, "action_config_both|") {
		return c.Send("QR Code и файл для "+configName, kb)
	}

	return c.Send("Непредвиденная ошибка.", kb)
}

func sanitizeConfigFileName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	return re.ReplaceAllString(name, "")
}

func HandleDownloadConfig(c telebot.Context) error {
	client := api.NewClient(c)

	configType := getConfigType(c)
	configName := getConfigName(c)
	kb := getActionConfigKeyboard(configType)

	fileData, apiError, err := client.GetConfigFile(configType, configName)
	if err != nil {
		if apiError != nil {
			fmt.Println("API error:", apiError.Message)
		} else {
			fmt.Println("Request error:", err)
		}
		return c.Send("Произошла ошибка при запросе файла.")
	}

	fileName := sanitizeConfigFileName(configName) + ".conf"

	doc := &telebot.Document{
		File: telebot.File{
			FileReader: bytes.NewReader(fileData),
		},
		FileName: fileName,
		Caption:  "Вот твой конфиг 😽",
	}

	return c.Send(doc, kb)
}

func HandleQrCodeConfig(c telebot.Context) error {
	client := api.NewClient(c)

	configType := getConfigType(c)
	configName := getConfigName(c)
	kb := getActionConfigKeyboard(configType)

	fileData, apiError, err := client.GetConfigQrCode(configType, configName)
	if err != nil {
		if apiError != nil {
			fmt.Println("API error:", apiError.Message)
		} else {
			fmt.Println("Request error:", err)
		}
		return c.Send("Произошла ошибка при запросе файла.")
	}

	photo := &telebot.Photo{
		File: telebot.File{
			FileReader: bytes.NewReader(fileData),
		},
		Caption: "Вот твой QR Code 😽",
	}

	return c.Send(photo, kb)
}

func HandleGetLinkConfig(c telebot.Context) error {
	client := api.NewClient(c)

	configType := getConfigType(c)
	configName := getConfigName(c)
	kb := getActionConfigKeyboard(configType)

	link, apiError, err := client.GetLink(configType, configName)
	if err != nil {
		if apiError != nil {
			fmt.Println("API error:", apiError.Message)
			return c.Send(apiError.Message, kb)
		} else {
			fmt.Println("Request error:", err)
		}
		return c.Send("Произошла ошибка при запросе ссылки.")
	}

	message := fmt.Sprintf("Вот твоя ссылка 😽\n\n`%s`", link)

	return c.Send(message, &telebot.SendOptions{
		ParseMode:   telebot.ModeMarkdown,
		ReplyMarkup: kb,
	})
}

func HandleBalance(c telebot.Context) error {
	kb := &telebot.ReplyMarkup{}

	btnPayment := kb.Data("Отправить запрос", "send_payment_request")
	btnToStart := kb.Data("К началу", "to_start")

	kb.Inline(kb.Row(btnPayment, btnToStart))

	client := api.NewClient(c)
	balance, err := client.GetBalance()

	if err != nil {
		return c.Send("An error occured.")
	}

	balanceString := `
*Баланс:* %.2f
*Долг:* %.2f

Деньги отправлять на Т-Банк по номеру +79230399748
 
После пополнения, отправьте запрос через команду по кнопке "Отправить запрос"
`
	balanceString = fmt.Sprintf(balanceString, balance.Amount, balance.Debt)
	balanceString = utils.EscapeMarkdownV2(balanceString)

	return c.Send(balanceString, &telebot.SendOptions{
		ParseMode:   telebot.ModeMarkdownV2,
		ReplyMarkup: kb,
	})
}

func HandleSendPaymentRequest(c telebot.Context) error {
	kb := &telebot.ReplyMarkup{}

	btnToStart := kb.Data("Отменить", "cancel_payment_and_return_to_start")

	waitingForAmount[c.Sender().ID] = true

	kb.Inline(kb.Row(btnToStart))

	return c.Send("На какую сумму хочешь пополнить?", kb)
}

func HandleSubmitPaymentRequest(c telebot.Context) error {
	data := strings.TrimSpace(c.Callback().Data)

	kb := &telebot.ReplyMarkup{}
	btnToStart := kb.Data("К началу", "to_start")

	stringAmount := strings.Replace(data, "submit_payment_request|", "", 1)
	amount, err := strconv.ParseFloat(stringAmount, 64)

	if err != nil {
		btnTryAgain := kb.Data("Повторить", "send_payment_request")
		kb.Inline(kb.Row(btnTryAgain, btnToStart))

		return c.Send("Неверное число.", kb)
	}

	kb.Inline(kb.Row(btnToStart))

	client := api.NewClient(c)

	response, _ := client.SendPaymentRequest(float32(amount))

	return c.Send(response.Message, kb)
}
