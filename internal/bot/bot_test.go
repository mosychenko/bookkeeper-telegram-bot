package bot

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestStartMessage(t *testing.T) {
	text, kb := StartMessage("https://budget.example")
	if text == "" || kb == nil {
		t.Fatal("пустое приветствие")
	}
	btn := kb.InlineKeyboard[0][0]
	if btn.Text != "Открыть" || btn.WebApp == nil || btn.WebApp.URL != "https://budget.example" {
		t.Fatalf("кнопка %+v", btn)
	}
	menu := MenuButton("https://budget.example")
	if menu.Text != "Бюджет" || menu.Type != models.MenuButtonTypeWebApp || menu.WebApp.URL != "https://budget.example" {
		t.Fatalf("меню %+v", menu)
	}
}

func TestIsStart(t *testing.T) {
	yes := []string{"/start", "/start args", "/start@MoneyBot"}
	for _, text := range yes {
		if !isStart(&models.Update{Message: &models.Message{Text: text}}) {
			t.Fatalf("%q должен быть /start", text)
		}
	}
	if isStart(&models.Update{Message: &models.Message{Text: "привет"}}) {
		t.Fatal("обычный текст")
	}
	if isStart(nil) || isStart(&models.Update{}) {
		t.Fatal("пустое обновление")
	}
}
