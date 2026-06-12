package commands

import (
	"fmt"
	"oksana-vpn-telegram-bot/pkg/api"
	"oksana-vpn-telegram-bot/pkg/utils"
	"strings"

	telebot "gopkg.in/telebot.v4"
)

var subscriptionPackages = []int{1, 3, 6, 12}

var subscriptionDiscounts = map[int]int{
	1:  0,
	3:  10,
	6:  20,
	12: 30,
}

func HandleBalance(c telebot.Context) error {
	kb := &telebot.ReplyMarkup{}

	btnPayment := kb.Data("Купить подписку", "send_payment_request")
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
 
Если баланса не хватит для выбранного пакета, бот создаст запрос на доплату и после подтверждения платежа подписка активируется автоматически.
`
	balanceString = fmt.Sprintf(balanceString, balance.Amount, balance.Debt, getSubscriptionDetails(status))
	balanceString = utils.EscapeMarkdownV2(balanceString)

	return c.Send(balanceString, &telebot.SendOptions{
		ParseMode:   telebot.ModeMarkdownV2,
		ReplyMarkup: kb,
	})
}

func HandleSendPaymentRequest(c telebot.Context) error {
	return c.Send(getSubscriptionPackagePrompt(), getSubscriptionPackageKeyboard())
}

func HandleChooseSubscriptionPackage(c telebot.Context) error {
	month, err := parseSubscriptionMonthCallback(c.Callback().Data, "choose_subscription_package|")
	if err != nil {
		kb := &telebot.ReplyMarkup{}
		btnRetry := kb.Data("Выбрать пакет", "send_payment_request")
		btnToStart := kb.Data("К началу", "to_start")
		kb.Inline(kb.Row(btnRetry, btnToStart))

		return c.Send("Неверный пакет подписки. Выбери один из доступных вариантов.", kb)
	}

	return c.Send(getSubscriptionBankPrompt(month), getSubscriptionBankKeyboard(month))
}

func HandleSubmitPaymentRequest(c telebot.Context) error {
	data := strings.TrimSpace(c.Callback().Data)

	kb := &telebot.ReplyMarkup{}
	btnToStart := kb.Data("К началу", "to_start")

	month, bank, err := parseSubscriptionSubmitCallback(data)
	if err != nil {
		btnTryAgain := kb.Data("Повторить", "send_payment_request")
		kb.Inline(kb.Row(btnTryAgain, btnToStart))
		return c.Send("Не получилось определить пакет подписки или банк. Попробуй ещё раз.", kb)
	}

	kb.Inline(kb.Row(btnToStart))

	client := api.NewClient(c)
	response, err := client.SendPaymentRequest(month, bank)
	if err != nil && api.IsMissingUserError(404, response.Message) {
		return c.Send(missingUserMessage(), kb)
	}

	return c.Send(buildSubscriptionPurchaseMessage(response), kb)
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

func getSubscriptionPackagePrompt() string {
	return `Доступные пакеты подписки:

1 месяц - скидка 0%
3 месяца - скидка 10%
6 месяцев - скидка 20%
12 месяцев - скидка 30%`
}

func getSubscriptionPackageKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}
	btnOne := kb.Data("1 месяц", "choose_subscription_package|1")
	btnThree := kb.Data("3 месяца", "choose_subscription_package|3")
	btnSix := kb.Data("6 месяцев", "choose_subscription_package|6")
	btnTwelve := kb.Data("12 месяцев", "choose_subscription_package|12")
	btnCancel := kb.Data("Отменить", "cancel_payment_and_return_to_start")

	kb.Inline(
		kb.Row(btnOne, btnThree),
		kb.Row(btnSix, btnTwelve),
		kb.Row(btnCancel),
	)

	return kb
}

func getSubscriptionBankPrompt(month int) string {
	return fmt.Sprintf("Выбран пакет на %s. Теперь выбери банк для оплаты.", formatSubscriptionDuration(month))
}

func getSubscriptionBankKeyboard(month int) *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}
	btnTBank := kb.Data("Т-Банк", fmt.Sprintf("submit_payment_request|%d|tbank", month))
	btnBack := kb.Data("Выбрать другой пакет", "send_payment_request")
	btnCancel := kb.Data("Отменить", "cancel_payment_and_return_to_start")

	kb.Inline(
		kb.Row(btnTBank),
		kb.Row(btnBack, btnCancel),
	)

	return kb
}

func parseSubscriptionMonthCallback(data string, prefix string) (int, error) {
	monthValue := strings.TrimSpace(strings.TrimPrefix(data, prefix))
	if monthValue == "" || monthValue == data {
		return 0, fmt.Errorf("missing month")
	}

	var month int
	if _, err := fmt.Sscanf(monthValue, "%d", &month); err != nil {
		return 0, err
	}

	if !isSupportedSubscriptionMonth(month) {
		return 0, fmt.Errorf("unsupported month")
	}

	return month, nil
}

func parseSubscriptionSubmitCallback(data string) (int, string, error) {
	parts := strings.Split(strings.TrimSpace(data), "|")
	if len(parts) != 3 || parts[0] != "submit_payment_request" {
		return 0, "", fmt.Errorf("invalid callback")
	}

	month, err := parseSubscriptionMonthCallback("choose_subscription_package|"+parts[1], "choose_subscription_package|")
	if err != nil {
		return 0, "", err
	}

	bank := strings.TrimSpace(parts[2])
	if bank == "" {
		return 0, "", fmt.Errorf("missing bank")
	}

	return month, bank, nil
}

func isSupportedSubscriptionMonth(month int) bool {
	for _, allowedMonth := range subscriptionPackages {
		if month == allowedMonth {
			return true
		}
	}

	return false
}

func formatSubscriptionDuration(month int) string {
	switch month {
	case 1:
		return "1 месяц"
	case 3:
		return "3 месяца"
	case 6:
		return "6 месяцев"
	case 12:
		return "12 месяцев"
	default:
		return fmt.Sprintf("%d мес.", month)
	}
}

func buildSubscriptionPurchaseMessage(response api.PaymentResponse) string {
	message := strings.TrimSpace(response.Message)

	switch response.Status {
	case "activated":
		if message == "" && response.FormattedEndDate != "" {
			message = fmt.Sprintf("Подписка активирована до %s.", response.FormattedEndDate)
		}
		if message == "" {
			message = "Подписка активирована."
		}
		return message + "\n\nПодписка уже активна и действует сразу."
	case "deposit_required":
		if message == "" && response.DepositAmount > 0 {
			message = fmt.Sprintf("Для активации подписки нужно пополнить баланс на %d.", response.DepositAmount)
		}
		if message == "" {
			message = "Для активации подписки нужно пополнить баланс."
		}
		return message + "\n\nПосле подтверждения оплаты подписка активируется автоматически."
	default:
		if message == "" {
			return "Что-то пошло не так :("
		}
		return message
	}
}
