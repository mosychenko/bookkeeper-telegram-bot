package sheet

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
)

func TestNovemberLayout(t *testing.T) {
	root := repoRoot(t)
	bf, err := LoadBudget(filepath.Join(root, "config", "budget.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	mp, err := LoadMapping(filepath.Join(root, "config", "sheets.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	nov := readLayoutCSV(t, "november.csv")
	oct := readLayoutCSV(t, "october.csv")
	ops := readLayoutCSV(t, "operations.csv")
	ops = append(ops, []string{
		"2026-11-02 09:00", "Ноябрь", "*1", "Яндекс Плюс", "", "-990", "Расход", "", "", "Подписки", "990", "Выполнено", "",
	})

	if nov[2][0] != "Баланс" || oct[2][0] != "Баланс" || nov[3][0] != "Доход" || oct[3][0] != "Доход" {
		t.Fatalf("строки съехали: ноябрь %q/%q октябрь %q/%q", nov[2][0], nov[3][0], oct[2][0], oct[3][0])
	}

	parsed := parseMonthly(nov, mp)
	if _, ok := parsed[norm("Кредит Максим")]; !ok {
		t.Fatal("перевод строки в «Кредит Максим» не схлопнулся")
	}
	if _, ok := parsed[norm("Кредиты:")]; ok {
		t.Fatal("заголовок «Кредиты:» не должен быть категорией")
	}
	if _, ok := parsed[norm("Карманные")]; ok {
		t.Fatal("заголовок «Карманные» не должен быть категорией")
	}
	known := map[string]bool{}
	for _, item := range bf.Mandatory {
		known[norm(item.Category)] = true
	}
	for _, item := range bf.Flexible {
		known[norm(item.Category)] = true
	}
	for _, item := range bf.Rare {
		known[norm(item.Category)] = true
	}
	for name := range parsed {
		if !known[name] {
			t.Errorf("категория %q есть в листе и нет в budget.yaml", parsed[name].Name)
		}
	}

	salaryNow := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	in, err := Assemble(salaryNow, bf, mp, ops, nov, oct)
	if err != nil {
		t.Fatal(err)
	}
	if in.Planned != 270000 {
		t.Fatalf("план зарплаты %d, это должна быть C4, не баланс C3 и не факт расходов", in.Planned)
	}
	if !in.BalanceKnown || in.Balance != 200000 {
		t.Fatalf("баланс %d known %v", in.Balance, in.BalanceKnown)
	}
	if !in.LastIncomeKnown || in.LastIncome != 252526 {
		t.Fatalf("октябрьская зарплата %d known %v, ждали 252526 из J3", in.LastIncome, in.LastIncomeKnown)
	}
	wantSalary := map[string]int64{
		"Коммунальные платежи":     3173,
		"Мобильная связь":          1500,
		"Карманные детям":          2000,
		"Школа":                    58000,
		"Психотерапевт":            15000,
		"Ипотека":                  47785,
		"Кредитка Женя":            11500,
		"Помещение":                27808,
		"Подписки":                 1000,
		"Бокс Женя":                4000,
		"Сбережения (Копилка 10%)": 27000,
	}
	absentSalary := []string{"Кредит Максим", "Маме", "Абика", "Кредиты Женя", "Интернет", "Корм питомцам", "Авто", "Образование (курсы)", "Страховка"}
	assertPayments(t, in.Mandatory, wantSalary, absentSalary)
	var subs, therapy, school, utilities, mortgage, mobile budget.Payment
	for _, p := range in.Mandatory {
		switch p.Name {
		case "Подписки":
			subs = p
		case "Психотерапевт":
			therapy = p
		case "Школа":
			school = p
		case "Коммунальные платежи":
			utilities = p
		case "Ипотека":
			mortgage = p
		case "Мобильная связь":
			mobile = p
		}
	}
	if !subs.Paid || subs.PaidNote != "990 ₽ Яндекс Плюс" || subs.Due != "20–23.11" {
		t.Fatalf("подписки %+v", subs)
	}
	if therapy.Due != "дата ?" {
		t.Fatalf("даты терапии %q", therapy.Due)
	}
	if school.Due != "6 и 25.11" {
		t.Fatalf("дата школы %q", school.Due)
	}
	if utilities.Due != "13.11" || mortgage.Due != "10 и 23.11" || mobile.Due != "7.11" {
		t.Fatalf("даты: коммунальные %q ипотека %q связь %q", utilities.Due, mortgage.Due, mobile.Due)
	}
	var cafe, courses budget.Flex
	var products bool
	for _, f := range in.Flexible {
		switch f.Name {
		case "Кафе":
			cafe = f
		case "Продукты":
			products = true
		case "Образование (курсы)":
			courses = f
		case "Подарки", "Путешествия", "Концерты", "Дизайн квартиры":
			t.Fatalf("редкая статья без плана и факта периода на экране: %+v", f)
		}
	}
	if cafe.Fact != 209 || cafe.Plan != nil || cafe.LastMonth != 28159 {
		t.Fatalf("кафе %+v", cafe)
	}
	if courses.Plan != nil || courses.Fact != 0 || courses.LastMonth != 7490 {
		t.Fatalf("образование %+v", courses)
	}
	if !products {
		t.Fatal("продукты с октябрьским фактом должны остаться на экране")
	}
	view := budget.Build(in)
	// Копилка 27 000 уже вычтена в C3, подписки 1 000 оплачены.
	// 198 766 − 27 000 − 1 000 = 170 766 ещё впереди. Баланс 200 000.
	// Дата баланса 10.11, до 20.11 остаётся 10 дней.
	if view.Mandatory.Total != 198766 || view.Free != 29234 || view.PerDay != 2923 || view.Days != 10 || view.StatusText != "Впритык" {
		t.Fatalf("ноябрь зарплата: обязательные %d свободно %d в день %d дней %d статус %s", view.Mandatory.Total, view.Free, view.PerDay, view.Days, view.StatusText)
	}
	if view.PerDayCaption != "можно тратить в день, 10 дней" {
		t.Fatalf("подпись дня %q", view.PerDayCaption)
	}

	advanceNow := time.Date(2026, time.November, 20, 12, 0, 0, 0, time.UTC)
	adv, err := Assemble(advanceNow, bf, mp, ops, nov, oct)
	if err != nil {
		t.Fatal(err)
	}
	if adv.Planned != 250000 {
		t.Fatalf("план аванса %d, ждали D4", adv.Planned)
	}
	if !adv.LastIncomeKnown || adv.LastIncome != 252150 {
		t.Fatalf("октябрьский аванс %d known %v, ждали J4", adv.LastIncome, adv.LastIncomeKnown)
	}
	wantAdvance := map[string]int64{
		"Коммунальные платежи":     10000,
		"Интернет":                 500,
		"Карманные детям":          2000,
		"Школа":                    105400,
		"Корм питомцам":            13000,
		"Психотерапевт":            15000,
		"Кредит Максим":            11477,
		"Маме":                     7500,
		"Авто":                     10000,
		"Подписки":                 5300,
		"Сбережения (Копилка 10%)": 25000,
	}
	assertPayments(t, adv.Mandatory, wantAdvance, []string{"Помещение", "Ипотека", "Абика", "Кредиты Женя", "Мобильная связь", "Бокс Женя"})
	var advUtilities, inet, feed, auto, pocket, loan budget.Payment
	for _, p := range adv.Mandatory {
		switch p.Name {
		case "Коммунальные платежи":
			advUtilities = p
		case "Интернет":
			inet = p
		case "Корм питомцам":
			feed = p
		case "Авто":
			auto = p
		case "Карманные детям":
			pocket = p
		case "Кредит Максим":
			loan = p
		}
	}
	if advUtilities.Due != "дата ?" || inet.Due != "28.11" || feed.Due != "22–31.11" || auto.Due != "по факту" || pocket.Due != "20–25.11" || loan.Due != "26.11" {
		t.Fatalf("даты аванса: коммунальные %q интернет %q корм %q авто %q карманные %q кредит %q", advUtilities.Due, inet.Due, feed.Due, auto.Due, pocket.Due, loan.Due)
	}
	advView := budget.Build(adv)
	if advView.Mandatory.Total != 205177 {
		t.Fatalf("обязательные аванса %d", advView.Mandatory.Total)
	}
	var still int64
	for _, p := range adv.Mandatory {
		if p.Name == "Сбережения (Копилка 10%)" && p.CoveredByBalance != p.Amount {
			t.Fatalf("копилка аванса должна уже быть в балансе: %+v", p)
		}
		if p.Paid || p.CoveredByBalance >= p.Amount {
			continue
		}
		still += p.Amount - p.CoveredByBalance
	}
	if advView.Free != adv.Balance-still {
		t.Fatalf("свободно аванса %d, баланс %d ещё заплатить %d", advView.Free, adv.Balance, still)
	}
	if advView.Days != 25 {
		t.Fatalf("дней аванса от даты баланса %d", advView.Days)
	}
	if !hasWarning(adv.Warnings, "Баланс в листе от") {
		t.Fatalf("старый баланс без предупреждения: %#v", adv.Warnings)
	}
}

func TestUnconfirmedKnobs(t *testing.T) {
	root := repoRoot(t)
	bf, err := LoadBudget(filepath.Join(root, "config", "budget.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var loans, school MandatoryItem
	var health, sections, courses bool
	for _, item := range bf.Mandatory {
		switch item.ID {
		case "wife_loans":
			loans = item
		case "school":
			school = item
		}
		if item.Category == "Образование (курсы)" {
			t.Fatal("образование должно быть гибким")
		}
	}
	for _, item := range bf.Flexible {
		switch item.Category {
		case "Здоровье":
			health = true
		case "Секции":
			sections = true
		case "Образование (курсы)":
			courses = item.OperationsName == "Образование"
		}
	}
	if !courses || !health || !sections {
		t.Fatal("образование, здоровье и секции должны быть гибкими")
	}
	if loans.AdvanceIfBlank != 0 || len(school.Periods) != 2 {
		t.Fatalf("ручки: кредиты %+v школа %+v", loans.AdvanceIfBlank, school.Periods)
	}

	mp, err := LoadMapping(filepath.Join(root, "config", "sheets.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	nov := readLayoutCSV(t, "november.csv")
	oct := readLayoutCSV(t, "october.csv")
	ops := readLayoutCSV(t, "operations.csv")
	edited := bf
	items := append([]MandatoryItem(nil), bf.Mandatory...)
	for i := range items {
		if items[i].ID == "wife_loans" {
			items[i].AdvanceIfBlank = 5000
		}
		if items[i].ID == "school" {
			items[i].Periods = []string{"salary"}
		}
	}
	edited.Mandatory = items
	now := time.Date(2026, time.November, 20, 12, 0, 0, 0, time.UTC)
	in, err := Assemble(now, edited, mp, ops, nov, oct)
	if err != nil {
		t.Fatal(err)
	}
	var sawLoans, sawSchool bool
	for _, p := range in.Mandatory {
		if p.Name == "Кредиты Женя" {
			sawLoans = p.Amount == 5000
		}
		if p.Name == "Школа" {
			sawSchool = true
		}
	}
	if !sawLoans || sawSchool {
		t.Fatalf("после правки конфига кредит %v школа %v", sawLoans, sawSchool)
	}
}

func assertPayments(t *testing.T, got []budget.Payment, want map[string]int64, absent []string) {
	t.Helper()
	have := map[string]budget.Payment{}
	for _, p := range got {
		have[p.Name] = p
	}
	if len(have) != len(want) {
		names := make([]string, 0, len(have))
		for name, p := range have {
			names = append(names, name+"="+strconv.FormatInt(p.Amount, 10))
		}
		t.Fatalf("платежей %d, ждали %d: %s", len(have), len(want), joinPayments(names))
	}
	for name, amount := range want {
		p, ok := have[name]
		if !ok || p.Amount != amount {
			t.Fatalf("%s: %+v, ждали %d", name, p, amount)
		}
	}
	for _, name := range absent {
		if p, ok := have[name]; ok {
			t.Fatalf("%s не должен быть в этом периоде: %+v", name, p)
		}
	}
}

func joinPayments(names []string) string {
	out := ""
	for i, name := range names {
		if i > 0 {
			out += ", "
		}
		out += name
	}
	return out
}

func hasWarning(warnings []string, part string) bool {
	for _, w := range warnings {
		if strings.Contains(w, part) {
			return true
		}
	}
	return false
}

func readLayoutCSV(t *testing.T, name string) [][]string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
