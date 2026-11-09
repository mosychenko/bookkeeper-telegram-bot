package sheet

import (
	"strings"
	"testing"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
)

func TestBalanceSkipsSavingsAndUsesRemainingDays(t *testing.T) {
	bf := savingsBudget()
	mp := incomeMapping()
	mp.Monthly.Balance = Balance{Row: 2, Label: "Баланс", LabelColumn: "A", SyncDate: "B", Column: "C"}
	grid := monthGrid(0, 99999, 80000)
	grid[1] = []string{"Баланс", "10.11.2026", "100000", "", ""}
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)

	in, err := Assemble(now, bf, mp, opsHeaderOnly(), grid, grid)
	if err != nil {
		t.Fatal(err)
	}
	if !in.BalanceKnown || in.Balance != 100000 {
		t.Fatalf("баланс %d known %v", in.Balance, in.BalanceKnown)
	}
	savings := payment(t, in.Mandatory, "Сбережения (Копилка 10%)")
	if savings.Amount != 27000 || savings.CoveredByBalance != 27000 {
		t.Fatalf("копилка %+v", savings)
	}
	view := budget.Build(in)
	if view.Free != 100000 || view.Days != 10 || view.PerDay != 10000 || view.PerDayCaption != "можно тратить в день, 10 дней" {
		t.Fatalf("свободно %d дни %d в день %d подпись %q", view.Free, view.Days, view.PerDay, view.PerDayCaption)
	}
	if view.Mandatory.Total != 27000 {
		t.Fatalf("итог обязательных %d", view.Mandatory.Total)
	}

	ops := [][]string{
		opsHeader(),
		incomeOp("06.11.2026", "Доход", "Зарплата", "100000"),
		incomeOp("06.11.2026", "Доход", "Аренда", "5000"),
		incomeOp("07.11.2026", "Доход", "Аренда", "22000"),
		incomeOp("06.11.2026", "Доход", "Прочие поступления", "40000"),
	}
	in, err = Assemble(now, bf, mp, ops, grid, grid)
	if err != nil {
		t.Fatal(err)
	}
	savings = payment(t, in.Mandatory, "Сбережения (Копилка 10%)")
	if savings.Amount != 10000+27000 || savings.CoveredByBalance != savings.Amount {
		t.Fatalf("копилка с арендой %+v", savings)
	}
	view = budget.Build(in)
	if view.Free != 100000 || view.Mandatory.Total != savings.Amount {
		t.Fatalf("аренда не должна вычитаться повторно: свободно %d итог %d", view.Free, view.Mandatory.Total)
	}
}

func TestEmptyBalanceFallsBackToPlan(t *testing.T) {
	bf := savingsBudget()
	mp := incomeMapping()
	mp.Monthly.Balance = Balance{Row: 2, Label: "Баланс", LabelColumn: "A", SyncDate: "B", Column: "C"}
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)

	for _, raw := range []string{"", "нет"} {
		grid := monthGrid(0, 99999, 0)
		grid[1] = []string{"Баланс", "01.11.2026", raw, "", ""}
		in, err := Assemble(now, bf, mp, opsHeaderOnly(), grid, grid)
		if err != nil {
			t.Fatal(err)
		}
		if in.BalanceKnown || in.Balance != 0 {
			t.Fatalf("ячейка %q не должна стать балансом %d known %v", raw, in.Balance, in.BalanceKnown)
		}
		view := budget.Build(in)
		if view.Free != 270000-27000 || view.Days != 15 || view.StatusText == "Нет" {
			t.Fatalf("откат для %q: свободно %d дни %d статус %s", raw, view.Free, view.Days, view.StatusText)
		}
		if !warningHas(in.Warnings, "пустой или не число") {
			t.Fatalf("нет предупреждения для %q: %#v", raw, in.Warnings)
		}
	}
}

func TestUnconfiguredBalanceFallsBack(t *testing.T) {
	bf := savingsBudget()
	mp := incomeMapping()
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	in, err := Assemble(now, bf, mp, opsHeaderOnly(), monthGrid(0, 0, 0), monthGrid(0, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if in.BalanceKnown {
		t.Fatal("баланс не настроен, но отмечен известным")
	}
	view := budget.Build(in)
	if view.Free != 270000-27000 || view.Days != 15 {
		t.Fatalf("откат: свободно %d дни %d", view.Free, view.Days)
	}
	if !warningHas(in.Warnings, "не задан") {
		t.Fatalf("предупреждение: %#v", in.Warnings)
	}
}

func TestStaleBalanceDateWarns(t *testing.T) {
	bf := savingsBudget()
	mp := incomeMapping()
	mp.Monthly.Balance = Balance{Row: 2, Label: "Баланс", LabelColumn: "A", SyncDate: "B", Column: "C"}
	grid := monthGrid(0, 0, 0)
	grid[1] = []string{"Баланс", "01.11.2026", "80000", "", ""}
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	in, err := Assemble(now, bf, mp, opsHeaderOnly(), grid, grid)
	if err != nil {
		t.Fatal(err)
	}
	if !in.BalanceKnown || in.BalanceAsOf.Day() != 1 {
		t.Fatalf("дата баланса %+v known %v", in.BalanceAsOf, in.BalanceKnown)
	}
	view := budget.Build(in)
	// 1.11 through 19.11 inclusive.
	if view.Days != 19 || view.Free != 80000 {
		t.Fatalf("от старой даты: дни %d свободно %d", view.Days, view.Free)
	}
	if !warningHas(in.Warnings, "от 1.11") || !warningHas(in.Warnings, "сегодня 10.11") {
		t.Fatalf("предупреждение: %#v", in.Warnings)
	}
}

func warningHas(warnings []string, part string) bool {
	for _, w := range warnings {
		if strings.Contains(w, part) {
			return true
		}
	}
	return false
}
