package sheet

import (
	"testing"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
)

const (
	piggyAccount = `Накопительный счет "Семейная копилка" *5348`
	mainAccount  = `Мастер-счет в рублях *4908`
)

func TestSavingsPaidWhenDepositsCoverTheLine(t *testing.T) {
	bf := markedSavings()
	bf.Flexible = []FlexibleItem{{Category: "Кафе"}}
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	ops := [][]string{opsHeader(),
		// Пара взноса: списание с обычного счёта не считается.
		piggyRow("05.11.2026", mainAccount, "Между своими счетами", "-27000", "", "Выполнено"),
		piggyRow("05.11.2026", piggyAccount, "Взнос в копилку", "27000", "27000", "Выполнено"),
		// Трата того же периода. Взнос «Исключено» в неё не входит.
		[]string{"06.11.2026", "Ноябрь", "*1", "Кофе", "", "100", "Расход", "", "", "Кафе", "100", "Выполнено", ""},
		// Взнос прошлого зарплатного периода не закрывает текущий,
		// но и не должен поднять темп гибких.
		piggyRow("10.10.2026", piggyAccount, "Взнос в копилку", "27000", "27000", "Выполнено"),
		[]string{"10.10.2026", "Октябрь", "*1", "Обед", "", "1500", "Расход", "", "", "Кафе", "1500", "Выполнено", ""},
	}
	mp := incomeMapping()
	mp.FactSource = "operations"
	in := assembleSavings(t, bf, mp, ops, monthGrid(0, 0, 0), now)
	savings := payment(t, in.Mandatory, "Сбережения (Копилка 10%)")
	if savings.Amount != 27000 || !savings.Paid || savings.PaidNote != "27 000 ₽ на копилку" {
		t.Fatalf("копилка %+v, ждали оплачено «27 000 ₽ на копилку»", savings)
	}
	view := budget.Build(in)
	if view.Free != 270000-27000 {
		t.Fatalf("при откате на план свободно %d: оплаченная копилка вычитается один раз", view.Free)
	}
	if view.Mandatory.Total != 27000 {
		t.Fatalf("итог обязательных %d, оплата не убирает строку из суммы", view.Mandatory.Total)
	}
	cafe := flexByName(t, in.Flexible, "Кафе")
	if cafe.Fact != 100 {
		t.Fatalf("факт кафе %d, взнос не должен войти в траты", cafe.Fact)
	}
	if !in.PaceKnown || in.LastFlexPerDay != 100 {
		t.Fatalf("темп %d known %v, ждали 1 500 / 15 дней без взноса", in.LastFlexPerDay, in.PaceKnown)
	}
	for _, row := range in.Top {
		if row.Name == "Взнос в копилку" || row.Amount == 27000 {
			t.Fatalf("взнос попал в топ: %+v", in.Top)
		}
	}
}

func TestSavingsPartialDepositStaysUnpaid(t *testing.T) {
	bf := markedSavings()
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	ops := [][]string{opsHeader(),
		piggyRow("05.11.2026", piggyAccount, "Взнос в копилку", "5000", "5000", "Выполнено"),
		// Незавершённый перевод не засчитывается.
		piggyRow("05.11.2026", piggyAccount, "Взнос в копилку", "22000", "22000", "В обработке"),
	}
	in := assembleSavings(t, bf, incomeMapping(), ops, monthGrid(0, 0, 0), now)
	savings := payment(t, in.Mandatory, "Сбережения (Копилка 10%)")
	if savings.Paid {
		t.Fatalf("частичный взнос не должен закрывать копилку: %+v", savings)
	}
	if savings.PaidNote != "внесено 5 000 ₽ из 27 000 ₽" {
		t.Fatalf("подпись частичного взноса %q", savings.PaidNote)
	}
}

func TestSavingsDepositOutsidePeriodDoesNotCount(t *testing.T) {
	bf := markedSavings()
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	ops := [][]string{opsHeader(),
		// 21 ноября — уже период аванса.
		piggyRow("21.11.2026", piggyAccount, "Взнос в копилку", "27000", "27000", "Выполнено"),
		// 4 ноября — последний день прошлого периода.
		piggyRow("04.11.2026", piggyAccount, "Взнос в копилку", "27000", "", "Выполнено"),
	}
	in := assembleSavings(t, bf, incomeMapping(), ops, monthGrid(0, 0, 0), now)
	savings := payment(t, in.Mandatory, "Сбережения (Копилка 10%)")
	if savings.Paid || savings.PaidNote != "" {
		t.Fatalf("взнос вне периода засчитан: %+v", savings)
	}
}

func TestSavingsRentOnPiggyAccountIsSummed(t *testing.T) {
	bf := markedSavings()
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	ops := [][]string{opsHeader(),
		incomeOp("05.11.2026", "Доход", "Зарплата", "252506"),
		incomeOp("05.11.2026", "Доход", "Аренда", "5000"),
		// 10% от 252 506 = 25 251, плюс аренда 5 000.
		piggyRow("05.11.2026", piggyAccount, "Взнос в копилку", "25251", "25251", "Выполнено"),
		piggyRow("05.11.2026", piggyAccount, "Взнос в копилку", "5000", "5 000", "Выполнено"),
	}
	in := assembleSavings(t, bf, incomeMapping(), ops, monthGrid(0, 0, 0), now)
	savings := payment(t, in.Mandatory, "Сбережения (Копилка 10%)")
	if savings.Amount != 30251 {
		t.Fatalf("сумма копилки %d, ждали 25 251 + аренда 5 000", savings.Amount)
	}
	if !savings.Paid || savings.PaidNote != "30 251 ₽ на копилку" {
		t.Fatalf("аренда и 10%% вместе: %+v", savings)
	}
}

func TestSavingsCategoryFallbackUsesAmount(t *testing.T) {
	bf := markedSavings()
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	// Счёт чужой, колонка M пустая: засчитывается категория, сумма — модуль F.
	ops := [][]string{opsHeader(),
		piggyRow("05.11.2026", "*9999", "Взнос в копилку", "-27000", "", "Выполнено"),
	}
	in := assembleSavings(t, bf, incomeMapping(), ops, monthGrid(0, 0, 0), now)
	savings := payment(t, in.Mandatory, "Сбережения (Копилка 10%)")
	if !savings.Paid || savings.PaidNote != "27 000 ₽ на копилку" {
		t.Fatalf("запасной признак: %+v", savings)
	}
}

func TestSavingsPaidDoesNotChangeFreeFromBalance(t *testing.T) {
	bf := markedSavings()
	mp := incomeMapping()
	mp.Monthly.Balance = Balance{Row: 2, Label: "Баланс", LabelColumn: "A", SyncDate: "B", Column: "C"}
	month := monthGrid(0, 0, 0)
	month[1] = []string{"Баланс", "10.11.2026", "100000", "", ""}
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	ops := [][]string{opsHeader(),
		piggyRow("05.11.2026", piggyAccount, "Взнос в копилку", "27000", "27000", "Выполнено"),
	}
	in := assembleSavings(t, bf, mp, ops, month, now)
	savings := payment(t, in.Mandatory, "Сбережения (Копилка 10%)")
	if !savings.Paid || savings.CoveredByBalance != savings.Amount {
		t.Fatalf("при известном балансе копилка %+v", savings)
	}
	view := budget.Build(in)
	if view.Free != 100000 {
		t.Fatalf("свободно %d: оплаченная копилка не должна вычитаться из баланса", view.Free)
	}
}

func markedSavings() BudgetFile {
	bf := savingsBudget()
	bf.SavingsAccountMarker = "*5348"
	bf.SavingsContributionCategory = "Взнос в копилку"
	return bf
}

func piggyRow(date, account, category, amount, movement, status string) []string {
	return []string{date, "Ноябрь", account, "Перевод", "", amount, "Исключено", "", "", category, "0", status, movement}
}

func assembleSavings(t *testing.T, bf BudgetFile, mp Mapping, ops [][]string, month [][]string, now time.Time) budget.Input {
	t.Helper()
	in, err := Assemble(now, bf, mp, ops, month, prevGrid(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func flexByName(t *testing.T, list []budget.Flex, name string) budget.Flex {
	t.Helper()
	for _, f := range list {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("нет статьи %s", name)
	return budget.Flex{}
}
