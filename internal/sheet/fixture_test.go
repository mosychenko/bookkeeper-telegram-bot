package sheet

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
)

func TestFixtureMatchesPrototype(t *testing.T) {
	root := repoRoot(t)
	in, err := LoadFixture(filepath.Join(root, "config", "budget.yaml"), filepath.Join(root, "config", "fixture.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	v := budget.Build(in)
	if v.Free != 46234 || v.PerDay != 3082 || v.StatusText != "Впритык" || v.LastPace != 3891 {
		t.Fatalf("фикстура разъехалась с прототипом: free %d day %d status %s pace %d", v.Free, v.PerDay, v.StatusText, v.LastPace)
	}
	if v.Question != "Хватит до 20.11?" || v.AsOf != "01.11" || v.PeriodLabel != "5.11 → 19.11" {
		t.Fatalf("шапка %+v", v)
	}
	if v.Mandatory.Total != 223766 || !v.Mandatory.Covered {
		t.Fatalf("обязательные %d %v", v.Mandatory.Total, v.Mandatory.Covered)
	}
	if len(v.Mandatory.Rows) != 11 {
		t.Fatalf("строк обязательных %d (интернет с нулём не должен попасть на экран)", len(v.Mandatory.Rows))
	}
	last := v.Mandatory.Rows[len(v.Mandatory.Rows)-1]
	if last.Name != "Подписки" || !last.Paid || last.PaidNote != "990 ₽ Яндекс Плюс" {
		t.Fatalf("подписки %+v", last)
	}
	if v.Flexible.Rows[2].Name != "Кафе" || v.Flexible.Rows[2].Fact != 209 || v.Flexible.Rows[2].Plan != nil {
		t.Fatalf("кафе %+v", v.Flexible.Rows[2])
	}
	want := "Перерасхода пока нет. Чтобы подсказка заработала, нужны лимиты по гибким статьям в листе «Ноябрь»."
	if v.Cover.Lines[0] != want {
		t.Fatalf("откуда взять %q", v.Cover.Lines[0])
	}
	if v.Top.Note != "Ноябрьский период ещё не начался, поэтому показан октябрь." {
		t.Fatalf("note %q", v.Top.Note)
	}
	if v.RiskText == "" || v.Top.Rows[0].Amount != 152800 {
		t.Fatalf("риск %q топ %+v", v.RiskText, v.Top.Rows[0])
	}
	wantRisk := "В октябре на гибкие траты уходило 3 891 ₽ в день. Если зарплата придёт как в октябре (252 526 ₽), а не 270 000 ₽ по плану, свободно 28 760 ₽, это 1 917 ₽ в день."
	if v.RiskText != wantRisk {
		t.Fatalf("риск:\n%s", v.RiskText)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestSheetsConfigLoads(t *testing.T) {
	m, err := LoadMapping(filepath.Join(repoRoot(t), "config", "sheets.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Operations.Columns["budget_category"] != "J" || m.Operations.Columns["fact"] != "K" {
		t.Fatalf("колонки операций %+v", m.Operations.Columns)
	}
	if m.Operations.ExpectHeaders["K"] != "Расход (для Факт)" || m.Operations.ExpectHeaders["M"] != "Копилка (движение)" {
		t.Fatalf("заголовки операций %+v", m.Operations.ExpectHeaders)
	}
	if m.Monthly.Columns["plan_salary"] != "C" || m.Monthly.Columns["plan_advance"] != "D" || m.Monthly.Columns["fact"] != "E" {
		t.Fatalf("колонки месяца %+v", m.Monthly.Columns)
	}
	if m.Monthly.IncomePlan.Row != 3 || m.Monthly.IncomePlan.SalaryColumn != "C" || m.Monthly.IncomePlan.AdvanceColumn != "D" {
		t.Fatalf("плановый доход %+v", m.Monthly.IncomePlan)
	}
	if m.Monthly.FirstDataRow != 5 || m.Monthly.ReadRange != "A1:J44" {
		t.Fatalf("диапазон месяца %+v", m.Monthly)
	}
	if m.Monthly.IncomeActual.Salary.Column != "J" || m.Monthly.IncomeActual.Advance.Row != 4 {
		t.Fatalf("фактический доход %+v", m.Monthly.IncomeActual)
	}
	bf, err := LoadBudget(filepath.Join(repoRoot(t), "config", "budget.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if bf.Income.Advance != 250000 || len(bf.PaidStatusValues) != 1 || bf.PaidStatusValues[0] != "Выполнено" {
		t.Fatalf("настройки бюджета %+v %v", bf.Income, bf.PaidStatusValues)
	}
}
