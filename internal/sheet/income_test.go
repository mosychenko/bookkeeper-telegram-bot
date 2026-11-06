package sheet

import (
	"strconv"
	"testing"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
)

func TestSalaryIncomeExcludesOtherAndRent(t *testing.T) {
	bf := savingsBudget()
	mp := incomeMapping()
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)

	// No salary rows in the operations: the J block says salary has not
	// arrived, while «Прочие поступления» and the loan are large.
	// Savings falls back to 10% of C3 and does not touch those amounts.
	in, err := Assemble(now, bf, mp, opsHeaderOnly(), monthGrid(0, 99999, 80000), prevGrid(252526, 51889))
	if err != nil {
		t.Fatal(err)
	}
	if in.Planned != 270000 {
		t.Fatalf("план %d, прочие не должны увеличивать доход", in.Planned)
	}
	if !in.LastIncomeKnown || in.LastIncome != 252526 {
		t.Fatalf("прошлый приход %d known %v, прочие октября не должны войти", in.LastIncome, in.LastIncomeKnown)
	}
	savings := payment(t, in.Mandatory, "Сбережения (Копилка 10%)")
	if savings.Amount != 27000 {
		t.Fatalf("копилка без прихода %d, ждали 10%% от плана 270 000", savings.Amount)
	}
	free := budget.Build(in)
	if free.Free != 270000-27000 {
		t.Fatalf("свободно %d", free.Free)
	}

	ops := [][]string{opsHeader()}
	ops = append(ops,
		incomeOp("06.11.2026", "Доход", "Зарплата", "100000"),
		incomeOp("06.11.2026", "Доход", "Отпускные", "20000"),
		incomeOp("08.11.2026", "Доход", "Доход Женя", "10000"),
		incomeOp("06.11.2026", "Доход", "Прочие поступления", "40000"),
		incomeOp("06.11.2026", "Доход", "Аренда", "5000"),
		incomeOp("07.11.2026", "Доход", "Аренда", "22000"),
		incomeOp("21.11.2026", "Доход", "Аванс", "50000"),
		incomeOp("06.11.2026", "Заём", "Заём у мамы", "80000"),
	)
	in, err = Assemble(now, bf, mp, ops, monthGrid(0, 99999, 80000), prevGrid(252526, 51889))
	if err != nil {
		t.Fatal(err)
	}
	if in.Planned != 270000 {
		t.Fatalf("план с операциями %d", in.Planned)
	}
	// 10% of 100000+20000+10000, plus rent 5000+22000. Advance is the other period.
	// Прочие and the loan stay out.
	savings = payment(t, in.Mandatory, "Сбережения (Копилка 10%)")
	if savings.Amount != 13000+27000 {
		t.Fatalf("копилка с фактом %d", savings.Amount)
	}

	advanceNow := time.Date(2026, time.November, 21, 12, 0, 0, 0, time.UTC)
	adv, err := Assemble(advanceNow, bf, mp, ops, monthGrid(0, 99999, 80000), prevGrid(252526, 51889))
	if err != nil {
		t.Fatal(err)
	}
	if adv.Planned != 250000 {
		t.Fatalf("план аванса %d", adv.Planned)
	}
	savings = payment(t, adv.Mandatory, "Сбережения (Копилка 10%)")
	if savings.Amount != 5000 {
		t.Fatalf("копилка аванса %d, ждали 10%% от аванса 50 000 без аренды другого периода", savings.Amount)
	}
}

func TestExcludedIncomeAloneDoesNotBecomeFree(t *testing.T) {
	bf := savingsBudget()
	mp := incomeMapping()
	ops := [][]string{
		opsHeader(),
		incomeOp("06.11.2026", "Доход", "Аренда", "22000"),
		incomeOp("06.11.2026", "Доход", "Прочие поступления", "40000"),
	}
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	in, err := Assemble(now, bf, mp, ops, monthGrid(0, 99999, 0), prevGrid(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	savings := payment(t, in.Mandatory, "Сбережения (Копилка 10%)")
	// No salary yet, so 10% of the plan, and the whole rent on top.
	if savings.Amount != 27000+22000 {
		t.Fatalf("копилка %d", savings.Amount)
	}
	if in.Planned != 270000 {
		t.Fatalf("план %d", in.Planned)
	}
	view := budget.Build(in)
	if view.Free != 270000-savings.Amount {
		t.Fatalf("свободно %d при копилке %d", view.Free, savings.Amount)
	}
}

func savingsBudget() BudgetFile {
	bf := BudgetFile{
		Mandatory: []MandatoryItem{{
			ID:       "savings",
			Name:     "Сбережения (Копилка 10%)",
			Category: "Сбережения (Копилка 10%)",
			Periods:  []string{"salary", "advance"},
		}},
		SalaryIncomeCategories:   []string{"Зарплата", "Отпускные", "Аванс", "Доход Женя"},
		ExcludedIncomeCategories: []string{"Аренда"},
		SavingsRate:              0.10,
		PaidStatusValues:         []string{"Выполнено"},
	}
	bf.Payday.SalaryDay = 5
	bf.Payday.AdvanceDay = 20
	bf.Income.Salary = 1
	bf.Income.Advance = 1
	return bf
}

func incomeMapping() Mapping {
	mp := baseMapping()
	mp.Monthly.FirstDataRow = 12
	mp.Monthly.IncomePlan = IncomePlan{
		Row: 3, Label: "Доход", LabelColumn: "A", SalaryColumn: "B", AdvanceColumn: "C",
	}
	mp.Monthly.IncomeActual = IncomeActual{
		Salary:  IncomeCell{Row: 3, Column: "E", Label: "Зарплата (вкл. отпускные)", LabelColumn: "D"},
		Advance: IncomeCell{Row: 4, Column: "E", Label: "Аванс", LabelColumn: "D"},
		Wife:    IncomeCell{Row: 5, Column: "E", Label: "Доход Женя", LabelColumn: "D"},
		Other:   IncomeCell{Row: 6, Column: "E", Label: "Прочие поступления", LabelColumn: "D"},
		Loan:    IncomeCell{Row: 8, Column: "E", Label: "Заём у мамы (не доход)", LabelColumn: "D"},
	}
	return mp
}

func monthGrid(salaryActual, other, loan int64) [][]string {
	row := func(a, b, c, d, e string) []string { return []string{a, b, c, d, e} }
	g := make([][]string, 8)
	g[0] = row("Категория", "С зарплаты", "С аванса", "Подпись", "Факт")
	g[1] = row("", "", "", "", "")
	g[2] = row("Доход", "270000", "250000", "Зарплата (вкл. отпускные)", strconv.FormatInt(salaryActual, 10))
	g[3] = row("Остаток", "", "", "Аванс", "0")
	g[4] = row("", "", "", "Доход Женя", "0")
	g[5] = row("", "", "", "Прочие поступления", strconv.FormatInt(other, 10))
	g[6] = row("", "", "", "", "")
	g[7] = row("", "", "", "Заём у мамы (не доход)", strconv.FormatInt(loan, 10))
	return g
}

func prevGrid(salary, other int64) [][]string {
	return monthGrid(salary, other, 0)
}

func opsHeader() []string {
	return []string{"Дата", "Месяц", "Счёт", "Описание", "Комментарий", "Сумма", "Тип", "Категория банка", "MCC", "Категория бюджета", "Расход (для факта)", "Статус", "Копилка"}
}

func opsHeaderOnly() [][]string {
	return [][]string{opsHeader()}
}

func incomeOp(date, typ, category, amount string) []string {
	return []string{date, "Ноябрь", "*1", "", "", amount, typ, "", "", category, "0", "Выполнено", ""}
}

func payment(t *testing.T, list []budget.Payment, name string) budget.Payment {
	t.Helper()
	for _, p := range list {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("нет платежа %s в %+v", name, list)
	return budget.Payment{}
}
