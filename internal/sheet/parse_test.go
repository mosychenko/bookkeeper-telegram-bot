package sheet

import (
	"strconv"
	"testing"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
)

func TestParseDateAndMoney(t *testing.T) {
	got, ok := parseDate("01.11.2026")
	if !ok || !got.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("дата %v %v", got, ok)
	}
	got, ok = parseDate("2026-05-01 13:12")
	if !ok || !got.Equal(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("дата со временем %v %v", got, ok)
	}
	want := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	serial := int(want.Sub(time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)).Hours() / 24)
	got, ok = parseDate(strconv.Itoa(serial))
	if !ok || !got.Equal(want) {
		t.Fatalf("серийная дата %d → %v", serial, got)
	}
	n, ok := parseMoney("46 234,40")
	if !ok || n != 46234 {
		t.Fatalf("деньги %d %v", n, ok)
	}
	n, ok = parseMoney("-1\u00a0500")
	if !ok || n != -1500 {
		t.Fatalf("минус %d", n)
	}
	if _, ok := parseMoney(""); ok {
		t.Fatal("пустая сумма не должна разбираться")
	}
}

func TestHeadersAndColumnMapping(t *testing.T) {
	mp := baseMapping()
	ops := [][]string{
		{"Дата", "Месяц", "Счёт", "Описание", "Комментарий", "Сумма", "Тип", "Категория банка", "MCC", "Категория бюджета", "Расход (для факта)", "Статус", "Копилка"},
		{"10.10.2026", "Октябрь", "*1", "Пятёрочка", "", "-100", "Расход", "Супермаркеты", "5411", "Продукты", "100", "", "да"},
	}
	if err := checkHeaders(ops, 1, mp.Operations.ExpectHeaders); err != nil {
		t.Fatal(err)
	}
	parsed, warns := parseOperations(ops, mp)
	if len(warns) != 0 || len(parsed) != 1 {
		t.Fatalf("ops %+v warns %v", parsed, warns)
	}
	if parsed[0].BudgetCategory != "Продукты" || parsed[0].Savings != "да" || parsed[0].Fact != 100 {
		t.Fatalf("разобрали не те колонки: %+v", parsed[0])
	}

	// The same numbers live in different columns. The mapping, not the
	// position we guessed first, decides what is the category.
	swapped := mp
	swapped.Operations.Columns = map[string]string{}
	for k, v := range mp.Operations.Columns {
		swapped.Operations.Columns[k] = v
	}
	swapped.Operations.Columns["budget_category"] = "A"
	swapped.Operations.Columns["fact"] = "B"
	swapped.Operations.Columns["date"] = "C"
	swapped.Operations.ExpectHeaders = map[string]string{"A": "Категория бюджета", "C": "Дата"}
	grid := [][]string{
		{"Категория бюджета", "Расход (для факта)", "Дата"},
		{"Кафе", "209", "02.11.2026"},
	}
	if err := checkHeaders(grid, 1, swapped.Operations.ExpectHeaders); err != nil {
		t.Fatal(err)
	}
	parsed, _ = parseOperations(grid, swapped)
	if parsed[0].BudgetCategory != "Кафе" || parsed[0].Fact != 209 || parsed[0].Date.Day() != 2 {
		t.Fatalf("колонки не следуют настройке: %+v", parsed[0])
	}

	broken := [][]string{{"Date", "Месяц"}}
	if err := checkHeaders(broken, 1, map[string]string{"A": "Дата"}); err == nil {
		t.Fatal("чужой заголовок должен останавливать чтение")
	}
}

func TestAssembleFollowsPlanColumnAndCover(t *testing.T) {
	bf := BudgetFile{
		Mandatory: []MandatoryItem{
			{ID: "subs", Name: "Подписки", Category: "Подписки", Period: "salary", AmountDefault: 1000, Due: &DueDay{Day: 1}},
			{ID: "school", Name: "Школа", Category: "Школа", Period: "salary", AmountDefault: 58000, Due: &DueDay{Day: 6, Approximate: true}},
			// A blank plan cell must not invent this default into the period.
			{ID: "inet", Name: "Интернет", Category: "Интернет", AmountDefault: 999},
		},
		Flexible:         []FlexibleItem{{Category: "Продукты"}, {Category: "Кафе"}},
		PaidStatusValues: []string{"оплачено"},
	}
	bf.Payday.SalaryDay = 5
	bf.Payday.AdvanceDay = 20
	bf.Income.Salary = 1 // must be overridden by the sheet

	mp := baseMapping()
	mp.Monthly.IncomeRows.Salary = "Зарплата"
	mp.Monthly.IncomeRows.Advance = "Аванс"
	mp.FactSource = "monthly"

	ops := [][]string{
		{"Дата", "Месяц", "Счёт", "Описание", "Комментарий", "Сумма", "Тип", "Категория банка", "MCC", "Категория бюджета", "Расход (для факта)", "Статус", "Копилка"},
		{"10.10.2026", "Октябрь", "*1", "Пятёрочка", "", "-58365", "Расход", "", "", "Продукты", "58365", "", ""},
		{"01.11.2026", "Ноябрь", "*1", "Яндекс Плюс", "", "-990", "Расход", "", "", "Подписки", "990", "оплачено", ""},
	}
	cur := [][]string{
		{"Категория", "С зарплаты", "С аванса", "Факт"},
		{"Продукты", "10000", "8000", "12000"},
		{"Кафе", "8000", "3000", "1000"},
		{"Зарплата", "270000", "90000", ""},
		{"Подписки", "1000", "", "990"},
		{"Школа", "58000", "", ""},
	}
	prev := [][]string{
		{"Категория", "С зарплаты", "С аванса", "Факт"},
		{"Продукты", "20000", "8000", "38161"},
		{"Кафе", "5000", "2000", "1000"},
		{"Зарплата", "252526", "80000", ""},
	}
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	in, err := Assemble(now, bf, mp, ops, cur, prev)
	if err != nil {
		t.Fatal(err)
	}
	if in.Planned != 270000 {
		t.Fatalf("план прихода %d", in.Planned)
	}
	if !in.LastIncomeKnown || in.LastIncome != 252526 {
		t.Fatalf("прошлый приход %d known %v", in.LastIncome, in.LastIncomeKnown)
	}
	if !in.PaceKnown || in.LastFlexPerDay != 3891 {
		t.Fatalf("темп %d known %v", in.LastFlexPerDay, in.PaceKnown)
	}
	// School cell is empty, so the configured default is used and reported.
	var school, subs budget.Payment
	for _, p := range in.Mandatory {
		switch p.Name {
		case "Школа":
			school = p
		case "Подписки":
			subs = p
		}
	}
	if school.Amount != 58000 || school.Due != "~6.11" {
		t.Fatalf("школа %+v", school)
	}
	for _, p := range in.Mandatory {
		if p.Name == "Интернет" {
			t.Fatalf("пустая ячейка не должна подставлять amount_default: %+v", p)
		}
	}
	if !subs.Paid || subs.PaidNote != "990 ₽ Яндекс Плюс" || subs.Due != "1.11" {
		t.Fatalf("подписки %+v", subs)
	}
	view := budget.Build(in)
	if len(view.Cover.Suggestions) != 1 {
		t.Fatalf("подсказки %+v строки %+v", view.Cover.Suggestions, view.Flexible.Rows)
	}
	s := view.Cover.Suggestions[0]
	if s.Category != "Продукты" || s.Overspend != 2000 || len(s.Takes) != 1 || s.Takes[0].From != "Кафе" || s.Takes[0].Amount != 2000 {
		t.Fatalf("откуда взять %+v", s)
	}
	var prod budget.Flex
	for _, f := range in.Flexible {
		if f.Name == "Продукты" {
			prod = f
		}
	}
	if prod.Plan == nil || *prod.Plan != 10000 || prod.Fact != 12000 || prod.LastMonth != 38161 {
		t.Fatalf("продукты %+v", prod)
	}

	// plan_salary is column A, category is column C. 20000 must win over 15000.
	swapped := mp
	swapped.Monthly.Columns = map[string]string{
		"plan_salary":  "A",
		"plan_advance": "B",
		"category":     "C",
		"fact":         "D",
	}
	swapped.Monthly.ExpectHeaders = nil
	cur2 := [][]string{
		{"план5", "план20", "имя", "факт"},
		{"20000", "15000", "Продукты", "100"},
		{"270000", "1", "Зарплата", ""},
	}
	prev2 := [][]string{
		{"план5", "план20", "имя", "факт"},
		{"1", "1", "Зарплата", ""},
	}
	in2, err := Assemble(now, bf, swapped, ops, cur2, prev2)
	if err != nil {
		t.Fatal(err)
	}
	if in2.Planned != 270000 {
		t.Fatalf("после перестановки колонок план %d", in2.Planned)
	}
	for _, f := range in2.Flexible {
		if f.Name == "Продукты" {
			if f.Plan == nil || *f.Plan != 20000 {
				t.Fatalf("план взяли не из колонки настройки: %+v", f)
			}
		}
	}
}

func baseMapping() Mapping {
	return Mapping{
		Operations: OperationsMap{
			Sheet:       "Операции",
			HeaderRow:   1,
			ReadColumns: "A:M",
			Columns: map[string]string{
				"date": "A", "month": "B", "account": "C", "description": "D",
				"comment": "E", "amount": "F", "type": "G", "bank_category": "H",
				"mcc": "I", "budget_category": "J", "fact": "K", "status": "L", "savings": "M",
			},
			ExpectHeaders: map[string]string{
				"A": "Дата", "B": "Месяц", "C": "Счёт", "D": "Описание", "E": "Комментарий",
				"F": "Сумма", "G": "Тип", "H": "Категория банка", "I": "MCC",
				"J": "Категория бюджета", "K": "Расход (для факта)", "L": "Статус", "M": "Копилка",
			},
		},
		Monthly: MonthlyMap{
			HeaderRow:    1,
			FirstDataRow: 2,
			ReadRange:    "A1:D50",
			Columns: map[string]string{
				"category": "A", "plan_salary": "B", "plan_advance": "C", "fact": "D",
			},
		},
		Types: TypeNames{
			Expense: "Расход", Excluded: "Исключено", Income: "Доход", Loan: "Заём", Cash: "Наличные",
		},
		FactSource:    "monthly",
		LastMonthFact: "monthly",
	}
}
