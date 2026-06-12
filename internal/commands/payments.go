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
	client := api.NewClient(c)
	packages, err := client.GetSubscriptionPackages()
	if err != nil {
		if api.IsMissingUserError(404, err.Error()) {
			return c.Send(missingUserMessage())
		}
		return c.Send("Не получилось загрузить пакеты подписки. Попробуй чуть позже.")
	}

	if len(packages) == 0 {
		return c.Send("Сейчас нет доступных пакетов подписки. Попробуй чуть позже.")
	}

	return c.Send(getSubscriptionPackagePrompt(packages), getSubscriptionPackageKeyboard(packages))
}

func HandleChooseSubscriptionPackage(c telebot.Context) error {
	month, err := parseSubscriptionMonthCallback(c.Callback().Data, "choose_subscription_package|")
	if err != nil {
		return c.Send("Неверный пакет подписки. Выбери один из доступных вариантов.", getRetrySubscriptionPackageKeyboard())
	}

	client := api.NewClient(c)
	packages, err := client.GetSubscriptionPackages()
	if err != nil {
		if api.IsMissingUserError(404, err.Error()) {
			return c.Send(missingUserMessage())
		}
		return c.Send("Не получилось проверить пакет подписки. Попробуй чуть позже.")
	}

	selectedPackage, ok := findSubscriptionPackage(packages, month)
	if !ok {
		return c.Send("Неверный пакет подписки. Выбери один из доступных вариантов.", getRetrySubscriptionPackageKeyboard())
	}

	return c.Send(getSubscriptionBankPrompt(selectedPackage), getSubscriptionBankKeyboard(month))
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

func getSubscriptionPackagePrompt(packages []api.SubscriptionPackage) string {
	lines := []string{"Доступные пакеты подписки:", ""}

	for _, subscriptionPackage := range packages {
		lines = append(lines, fmt.Sprintf(
			"%s - %d ₽ (скидка %d%%)",
			formatSubscriptionDuration(subscriptionPackage.Month),
			subscriptionPackage.Price,
			subscriptionPackage.DiscountPercent,
		))
	}

	return strings.Join(lines, "\n")
}

func getSubscriptionPackageKeyboard(packages []api.SubscriptionPackage) *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}
	btnCancel := kb.Data("Отменить", "cancel_payment_and_return_to_start")

	var rows []telebot.Row
	for i := 0; i < len(packages); i += 2 {
		buttons := []telebot.Btn{
			kb.Data(
				fmt.Sprintf("%s - %d ₽", formatSubscriptionDuration(packages[i].Month), packages[i].Price),
				fmt.Sprintf("choose_subscription_package|%d", packages[i].Month),
			),
		}

		if i+1 < len(packages) {
			buttons = append(buttons, kb.Data(
				fmt.Sprintf("%s - %d ₽", formatSubscriptionDuration(packages[i+1].Month), packages[i+1].Price),
				fmt.Sprintf("choose_subscription_package|%d", packages[i+1].Month),
			))
		}

		rows = append(rows, kb.Row(buttons...))
	}

	rows = append(rows, kb.Row(btnCancel))
	kb.Inline(rows...)

	return kb
}

func getSubscriptionBankPrompt(subscriptionPackage api.SubscriptionPackage) string {
	return fmt.Sprintf(
		"Выбран пакет на %s за %d ₽. Теперь выбери банк для оплаты.",
		formatSubscriptionDuration(subscriptionPackage.Month),
		subscriptionPackage.Price,
	)
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

	if month <= 0 {
		return 0, fmt.Errorf("unsupported month")
	}

	return month, nil
}

func getRetrySubscriptionPackageKeyboard() *telebot.ReplyMarkup {
	kb := &telebot.ReplyMarkup{}
	btnRetry := kb.Data("Выбрать пакет", "send_payment_request")
	btnToStart := kb.Data("К началу", "to_start")
	kb.Inline(kb.Row(btnRetry, btnToStart))

	return kb
}

func parseSubscriptionSubmitCallback(data string) (int, string, error) {
	parts := strings.Split(strings.TrimSpace(data), "|")
	if len(parts) != 3 || parts[0] != "submit_payment_request" {
		return 0, "", fmt.Errorf("invalid callback")
	}

	month, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, "", err
	}
	if month <= 0 {
		return 0, "", fmt.Errorf("unsupported month")
	}

	bank := strings.TrimSpace(parts[2])
	if bank == "" {
		return 0, "", fmt.Errorf("missing bank")
	}

	return month, bank, nil
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

func findSubscriptionPackage(packages []api.SubscriptionPackage, month int) (api.SubscriptionPackage, bool) {
	for _, subscriptionPackage := range packages {
		if subscriptionPackage.Month == month {
			return subscriptionPackage, true
		}
	}

	return api.SubscriptionPackage{}, false
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
