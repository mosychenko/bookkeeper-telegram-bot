package bot

import (
	"context"
	"log/slog"
	"strings"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/mosychenko/bookkeeper-telegram-bot/internal/auth"
)

const menuText = "Бюджет"
const buttonText = "Открыть"

// MenuButton is the chat menu button that opens the Mini App.
func MenuButton(webAppURL string) *models.MenuButtonWebApp {
	return &models.MenuButtonWebApp{
		Type:   models.MenuButtonTypeWebApp,
		Text:   menuText,
		WebApp: models.WebAppInfo{URL: webAppURL},
	}
}

// StartMessage is the /start reply: a short line and a button into the Mini App.
func StartMessage(webAppURL string) (string, *models.InlineKeyboardMarkup) {
	text := "Деньги до зарплаты.\nОдин экран: хватит ли денег до следующей выплаты и сколько можно тратить в день."
	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: buttonText, WebApp: &models.WebAppInfo{URL: webAppURL}},
		}},
	}
	return text, kb
}

// Run polls Telegram. /start answers with the Mini App button and the menu
// button is set to the same web app. Callers must pass a non-empty token;
// tests do not call Run.
func Run(ctx context.Context, token, webAppURL string, allow auth.Allowlist) error {
	b, err := tg.New(token)
	if err != nil {
		return err
	}
	if _, err := b.SetChatMenuButton(ctx, &tg.SetChatMenuButtonParams{MenuButton: MenuButton(webAppURL)}); err != nil {
		// /start still sets the button for the chat that opened the bot.
		slog.Error("кнопка меню", "err", err)
	}
	b.RegisterHandlerMatchFunc(isStart, func(ctx context.Context, b *tg.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}
		chatID := update.Message.Chat.ID
		if !allow.Allows(update.Message.From.ID) {
			_, _ = b.SendMessage(ctx, &tg.SendMessageParams{
				ChatID: chatID,
				Text:   "Этот бот только для своей семьи.",
			})
			return
		}
		_, _ = b.SetChatMenuButton(ctx, &tg.SetChatMenuButtonParams{
			ChatID:     chatID,
			MenuButton: MenuButton(webAppURL),
		})
		text, kb := StartMessage(webAppURL)
		_, _ = b.SendMessage(ctx, &tg.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: kb,
		})
	})
	b.Start(ctx)
	return nil
}

func isStart(update *models.Update) bool {
	if update == nil || update.Message == nil {
		return false
	}
	text := strings.TrimSpace(update.Message.Text)
	return text == "/start" || strings.HasPrefix(text, "/start ") || strings.HasPrefix(text, "/start@")
}
