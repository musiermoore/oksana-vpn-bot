package commands

import (
	"fmt"
	"oksana-vpn-telegram-bot/pkg/api"
	"oksana-vpn-telegram-bot/pkg/models"
	"oksana-vpn-telegram-bot/pkg/utils"
	"strconv"
	"strings"

	telebot "gopkg.in/telebot.v4"
)

var waitingForAmount = make(map[int64]bool)

func RegisterCommands(bot *telebot.Bot) {
	menu := &telebot.ReplyMarkup{}

	btnConfigs := menu.Data("Конфиги", "menu_configs")
	btnBalance := menu.Data("Баланс", "menu_balance")
	btnHelp := menu.Data("Помощь", "menu_help")
	menu.Inline(
		menu.Row(btnConfigs, btnBalance, btnHelp),
	)

	bot.Handle("/start", func(c telebot.Context) error {
		return c.Send("Привет! Выбери команду:", menu)
	})

	bot.Handle("/help", HandleHelpCommand)
	bot.Handle("/configs", HandleConfigsCommand)

	// Handle main menu buttons
	bot.Handle(&btnConfigs, HandleConfigsButton)
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
			return c.Edit("Выбери команду:", menu)
		} else if data == "to_configs" {
			return HandleConfigsButton(c)
		} else if strings.HasPrefix(data, "send_payment_request") {
			return HandleSendPaymentRequest(c)
		} else if strings.HasPrefix(data, "submit_payment_request|") {
			return HandleSubmitPaymentRequest(c)
		}
		// handle action callbacks similarly...
		return c.Respond()
	})

	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		userId := c.Sender().ID
		text := c.Message().Text

		// If waiting for amount
		if waitingForAmount[userId] {
			waitingForAmount[userId] = false

			amount, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return c.Send("Please enter a valid number.")
			}

			kb := &telebot.ReplyMarkup{}
			btnAdd := kb.Data("Add", fmt.Sprintf("submit_payment_request|%f", amount))
			btnCancel := kb.Data("Cancel", "to_start")
			kb.Inline(kb.Row(btnAdd, btnCancel))

			// Handle the amount
			return c.Send(fmt.Sprintf("✅ You want to add $%.2f?", amount), kb)
		}

		// If not waiting — handle normal text
		return c.Send("Use /start to show menu")
	})
}

func getConfigsKeyboard(c telebot.Context) *telebot.ReplyMarkup {
	configs := []models.Config{
		{ID: 1, Name: "Config 1"},
		{ID: 2, Name: "Config 2"},
		{ID: 3, Name: "Config 3"},
	}
	kb := &telebot.ReplyMarkup{}
	row := []telebot.Btn{}
	for _, config := range configs {
		row = append(row, kb.Data(config.Name, "config|"+strconv.Itoa(config.ID)+"|"+config.Name))
	}

	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(
		kb.Row(row...),
		kb.Row(btnToStart),
	)

	return kb
}

func HandleConfigsCommand(c telebot.Context) error {
	kb := getConfigsKeyboard(c)

	return c.Send("Выбери конфиг:", kb)
}

func HandleConfigsButton(c telebot.Context) error {
	kb := getConfigsKeyboard(c)

	return c.Edit("Выбери конфиг:", kb)
}

func getHelpData() (*telebot.ReplyMarkup, string) {
	kb := &telebot.ReplyMarkup{}

	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(kb.Row(btnToStart))

	help := "Очень полезный текст"

	return kb, help
}

func HandleHelpCommand(c telebot.Context) error {
	kb, message := getHelpData()

	return c.Send(message, kb)
}

func HandleHelpButton(c telebot.Context) error {
	kb, message := getHelpData()

	return c.Edit(message, kb)
}

func HandleChoosingConfig(c telebot.Context) error {
	config := strings.TrimSpace(c.Callback().Data)

	configName := strings.Split(config, "|")[2]

	kb := &telebot.ReplyMarkup{}

	btnQR := kb.Data("QR Code", "action_config_qr|"+config)
	btnDownload := kb.Data("Файл", "action_config_file|"+config)
	btnBoth := kb.Data("QR и файл", "action_config_both|"+config)
	btnConfigs := kb.Data("Конфиги", "to_configs")

	kb.Inline(
		kb.Row(btnQR, btnDownload, btnBoth),
		kb.Row(btnConfigs),
	)

	return c.Edit("Выбери действие для конфига "+configName, kb)
}

func HandleActionConfig(c telebot.Context) error {
	data := strings.TrimSpace(c.Callback().Data)
	data = strings.Replace(data, "config|", "", 1)

	configName := strings.Split(data, "|")[2]

	kb := &telebot.ReplyMarkup{}
	btnPrev := kb.Data("Конфиги", "to_configs")
	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(kb.Row(btnPrev, btnToStart))

	fmt.Println(data)
	if strings.HasPrefix(data, "action_config_qr|") {
		return c.Edit("QR Code для "+configName, kb)
	} else if strings.HasPrefix(data, "action_config_file|") {
		return c.Edit("Файл для "+configName, kb)
	} else if strings.HasPrefix(data, "action_config_both|") {
		return c.Edit("QR Code и файл для "+configName, kb)
	}

	return c.Edit("Непредвиденная ошибка.", kb)
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

	return c.Edit(balanceString, &telebot.SendOptions{
		ParseMode:   telebot.ModeMarkdownV2,
		ReplyMarkup: kb,
	})
}

func HandleSendPaymentRequest(c telebot.Context) error {
	kb := &telebot.ReplyMarkup{}

	btnToStart := kb.Data("К началу", "to_start")

	waitingForAmount[c.Sender().ID] = true

	kb.Inline(kb.Row(btnToStart))

	return c.Edit("На какую сумму хочешь пополнить?", kb)
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

	return c.Edit(fmt.Sprintf("Запрос на %.2f отправлен.", amount), kb)
}
