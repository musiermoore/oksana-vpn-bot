package commands

import (
	"fmt"
	"oksana-vpn-telegram-bot/pkg/api"
	"oksana-vpn-telegram-bot/pkg/utils"
	"strconv"
	"strings"

	telebot "gopkg.in/telebot.v4"
)

func HandleBalance(c telebot.Context) error {
	kb := &telebot.ReplyMarkup{}

	btnPayment := kb.Data("Отправить запрос", "send_payment_request")
	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(kb.Row(btnPayment, btnToStart))

	client := api.NewClient(c)
	balance, err := client.GetBalance()
	if err != nil {
		if api.IsMissingUserError(404, err.Error()) {
			return c.Send(missingUserMessage())
		}
		return c.Send("Произошла ошибка. Попробуй чуть позже.")
	}

	status, err := client.GetRegistrationStatus()
	if err != nil {
		return c.Send("Произошла ошибка. Попробуй чуть позже.")
	}

	balanceString := `
*Баланс:* %.2f
*Долг:* %.2f
%s

Деньги отправлять на Т-Банк по номеру +79230399748
 
После пополнения, отправьте запрос через команду по кнопке "Отправить запрос"
`
	balanceString = fmt.Sprintf(balanceString, balance.Amount, balance.Debt, getSubscriptionDetails(status))
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
	response, err := client.SendPaymentRequest(float32(amount))
	if err != nil && api.IsMissingUserError(404, response.Message) {
		return c.Send(missingUserMessage(), kb)
	}

	return c.Send(response.Message, kb)
}

func HandleDepositAction(c telebot.Context) error {
	data := strings.TrimSpace(c.Callback().Data)
	parts := strings.SplitN(data, "|", 2)
	if len(parts) != 2 || parts[1] == "" {
		_ = c.Respond(&telebot.CallbackResponse{})
		return c.Send("Unknown action.")
	}

	action := parts[0]
	transactionID := parts[1]
	client := api.NewClient(c)

	if err := c.Respond(&telebot.CallbackResponse{}); err != nil {
		return err
	}

	username := c.Sender().Username
	if username == "" {
		username = c.Sender().FirstName
	}

	var err error
	var responseText string

	switch action {
	case "approve_deposit":
		err = client.ApprovePaymentRequest(transactionID)
		if err == nil {
			responseText = fmt.Sprintf("Запрос @%s принят ✅", username)
		} else {
			responseText = "Ошибка =("
		}
	case "deny_deposit":
		err = client.DeclinePaymentRequest(transactionID)
		if err == nil {
			responseText = fmt.Sprintf("Запрос @%s отклонён ❌", username)
		} else {
			responseText = "Ошибка =("
		}
	default:
		responseText = "Unknown action."
	}

	return c.Send(responseText)
}
