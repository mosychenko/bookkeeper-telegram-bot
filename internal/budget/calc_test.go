package budget

import (
	"testing"
	"time"
)

func novSalaryPeriod(t *testing.T) Period {
	t.Helper()
	p, err := PeriodOf(2026, time.November, KindSalary, 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func ptr(n int64) *int64 { return &n }

// prototypeInput is the approved November screen: data as of 1 Nov,
// salary period 5.11-19.11, before it has started.
func prototypeInput(t *testing.T) Input {
	t.Helper()
	asOf := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	return Input{
		Now:             asOf,
		AsOf:            asOf,
		Period:          novSalaryPeriod(t),
		Demo:            true,
		Planned:         270000,
		LastIncome:      252526,
		LastIncomeKnown: true,
		LastFlexPerDay:  3891,
		PaceKnown:       true,
		Mandatory: []Payment{
			{Name: "Копилка 10%", Amount: 52000, Due: "5.11"},
			{Name: "Школа", Amount: 58000, Due: "~6.11"},
			{Name: "Мобильная связь", Amount: 1500, Due: "~7.11"},
			{Name: "Ипотека + страховка", Amount: 47785, Due: "~10.11"},
			{Name: "Психотерапевт", Amount: 15000, Due: "11 и 18.11"},
			{Name: "Кредит за помещение", Amount: 27808, Due: "~12.11"},
			{Name: "Коммунальные", Amount: 3173, Due: "~13.11"},
			{Name: "Кредитка Женя", Amount: 11500, Due: "дата ?"},
			{Name: "Бокс Женя", Amount: 4000, Due: "дата ?"},
			{Name: "Карманные детям", Amount: 2000, Due: "дата ?"},
			{Name: "Подписки", Amount: 1000, Due: "1.11", Paid: true, PaidNote: "990 ₽ Яндекс Плюс"},
		},
		Flexible: []Flex{
			{Name: "Продукты", Fact: 0, LastMonth: 38161},
			{Name: "Прочее", Fact: 0, LastMonth: 34667},
			{Name: "Кафе", Fact: 209, LastMonth: 28159},
			{Name: "Курилка", Fact: 0, LastMonth: 7966},
			{Name: "Здоровье", Fact: 0, LastMonth: 5763},
			{Name: "Транспорт", Fact: 0, LastMonth: 4040},
		},
		Top: []Spent{
			{Name: "Школа (вкл. предоплату ноября)", Amount: 152800},
			{Name: "Ипотека", Amount: 47785},
			{Name: "Продукты", Amount: 38161},
			{Name: "Подарки", Amount: 37598},
			{Name: "Прочее", Amount: 34667},
		},
	}
}

func TestPrototypeNovember(t *testing.T) {
	v := Build(prototypeInput(t))

	if v.Free != 46234 {
		t.Fatalf("свободно = %d, хотели 46234", v.Free)
	}
	if v.PerDay != 3082 {
		t.Fatalf("в день = %d, хотели 3082", v.PerDay)
	}
	if v.StatusText != "Впритык" || v.Status != "tight" {
		t.Fatalf("статус = %s/%s, хотели Впритык", v.Status, v.StatusText)
	}
	if v.LastPace != 3891 || !v.PaceKnown {
		t.Fatalf("темп прошлого месяца = %d known=%v", v.LastPace, v.PaceKnown)
	}
	if v.Days != 15 {
		t.Fatalf("дней = %d", v.Days)
	}
	if v.Question != "Хватит до 20.11?" {
		t.Fatalf("вопрос %q", v.Question)
	}
	if v.PeriodLabel != "5.11 → 19.11" {
		t.Fatalf("период %q", v.PeriodLabel)
	}
	if v.AsOf != "01.11" {
		t.Fatalf("дата данных %q", v.AsOf)
	}
	if v.Mandatory.Total != 223766 {
		t.Fatalf("обязательные = %d, хотели 223766", v.Mandatory.Total)
	}
	if !v.Mandatory.Covered || v.Mandatory.TotalText != "денег хватает" {
		t.Fatalf("итог обязательных: %+v", v.Mandatory)
	}
	// Paid subscriptions still count toward the obligation, as in the prototype.
	if !v.Mandatory.Rows[len(v.Mandatory.Rows)-1].Paid {
		t.Fatal("подписки должны быть оплачены")
	}

	wantRisk := "В октябре на гибкие траты уходило 3 891 ₽ в день. Если зарплата придёт как в октябре (252 526 ₽), а не 270 000 ₽ по плану, свободно 28 760 ₽, это 1 917 ₽ в день."
	if v.RiskText != wantRisk {
		t.Fatalf("риск:\n got %s\nwant %s", v.RiskText, wantRisk)
	}
	if v.Mandatory.Caption != "Даты с «~» — как списывалось в октябре" {
		t.Fatalf("подпись обязательных %q", v.Mandatory.Caption)
	}
	if v.Flexible.Caption != "В плане ноября лимиты не заданы: всё идёт из свободного остатка" {
		t.Fatalf("подпись гибких %q", v.Flexible.Caption)
	}
	if v.Flexible.PrevMonth != "Октябрь" {
		t.Fatalf("прошлый месяц %q", v.Flexible.PrevMonth)
	}
	if len(v.Flexible.Rows) != 6 || v.Flexible.Rows[2].Name != "Кафе" || v.Flexible.Rows[2].Fact != 209 {
		t.Fatalf("гибкие строки %+v", v.Flexible.Rows)
	}
	if v.Flexible.Rows[0].Plan != nil || v.Flexible.Rows[0].Left != nil {
		t.Fatal("план в прототипе не задан")
	}
	wantCover := "Перерасхода пока нет. Чтобы подсказка заработала, нужны лимиты по гибким статьям в листе «Ноябрь»."
	if len(v.Cover.Lines) != 1 || v.Cover.Lines[0] != wantCover {
		t.Fatalf("откуда взять: %#v", v.Cover.Lines)
	}
	if len(v.Cover.Suggestions) != 0 {
		t.Fatalf("подсказок быть не должно: %+v", v.Cover.Suggestions)
	}
	if v.Top.Column != "Топ-5 октября" {
		t.Fatalf("колонка топа %q", v.Top.Column)
	}
	wantNote := "Ноябрьский период ещё не начался, поэтому показан октябрь."
	if v.Top.Note != wantNote {
		t.Fatalf("примечание %q", v.Top.Note)
	}
	if v.Top.Rows[0].Amount != 152800 {
		t.Fatalf("топ %+v", v.Top.Rows)
	}
	if v.PerDayCaption != "можно тратить в день, 15 дней" {
		t.Fatalf("подпись дня %q", v.PerDayCaption)
	}
}

func TestStatusThresholds(t *testing.T) {
	base := prototypeInput(t)
	base.Demo = false

	t.Run("negative free is no", func(t *testing.T) {
		in := base
		in.Planned = 1000
		v := Build(in)
		if v.StatusText != "Нет" || v.Free >= 0 {
			t.Fatalf("status %s free %d", v.StatusText, v.Free)
		}
		if v.Mandatory.Covered || v.Mandatory.TotalText != "не хватает "+FormatRUB(-v.Free) {
			t.Fatalf("итог %q shortfall %d", v.Mandatory.TotalText, v.Mandatory.Shortfall)
		}
	})

	t.Run("pace equal is yes", func(t *testing.T) {
		in := base
		// free/days == 3891 exactly → not strictly below.
		in.Planned = in.MandatoryTotal() + 3891*15
		v := Build(in)
		if v.StatusText != "Да" {
			t.Fatalf("status %s free %d perDay %d", v.StatusText, v.Free, v.PerDay)
		}
		if v.Free != 3891*15 || v.PerDay != 3891 {
			t.Fatalf("free %d perDay %d", v.Free, v.PerDay)
		}
	})

	t.Run("one ruble under the pace is tight", func(t *testing.T) {
		in := base
		in.Planned = in.MandatoryTotal() + 3891*15 - 1
		v := Build(in)
		if v.StatusText != "Впритык" {
			t.Fatalf("status %s free %d", v.StatusText, v.Free)
		}
	})

	t.Run("above the pace is yes", func(t *testing.T) {
		in := base
		in.Planned = in.MandatoryTotal() + 5000*15
		v := Build(in)
		if v.StatusText != "Да" || v.PerDay != 5000 {
			t.Fatalf("status %s perDay %d", v.StatusText, v.PerDay)
		}
	})

	t.Run("unknown pace does not invent tight", func(t *testing.T) {
		in := base
		in.PaceKnown = false
		in.LastFlexPerDay = 0
		v := Build(in)
		if v.StatusText != "Да" {
			t.Fatalf("status %s", v.StatusText)
		}
		if v.RiskText == "" || contains(v.RiskText, "уходило") {
			t.Fatalf("риск без темпа: %s", v.RiskText)
		}
	})
}

func (in Input) MandatoryTotal() int64 {
	var n int64
	for _, p := range in.Mandatory {
		n += p.Amount
	}
	return n
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestOverspendAndCover(t *testing.T) {
	in := prototypeInput(t)
	in.Flexible = []Flex{
		{Name: "Продукты", Plan: ptr(10000), Fact: 18000, LastMonth: 38161}, // over 8000
		{Name: "Кафе", Plan: ptr(5000), Fact: 7000, LastMonth: 1000},        // over 2000
		{Name: "Транспорт", Plan: ptr(4000), Fact: 1000, LastMonth: 500},    // left 3000
		{Name: "Прочее", Plan: ptr(9000), Fact: 1000, LastMonth: 400},       // left 8000
		{Name: "Курилка", Plan: ptr(1000), Fact: 1000, LastMonth: 100},      // left 0
		{Name: "Здоровье", Fact: 100, LastMonth: 50},                        // no plan
	}
	v := Build(in)

	if v.Flexible.Rows[0].Name != "Продукты" || !v.Flexible.Rows[0].Overspent {
		t.Fatalf("сначала самый большой перерасход, получили %+v", v.Flexible.Rows[0])
	}
	if v.Flexible.Rows[1].Name != "Кафе" || !v.Flexible.Rows[1].Overspent {
		t.Fatalf("вторая строка %+v", v.Flexible.Rows[1])
	}
	// Unset plan stays after the overspent rows, in its original relative place.
	var health int
	for i, r := range v.Flexible.Rows {
		if r.Name == "Здоровье" {
			health = i
			if r.Plan != nil || r.Left != nil {
				t.Fatalf("у здоровья не должно быть плана: %+v", r)
			}
		}
	}
	if health < 2 {
		t.Fatalf("здоровье без плана уехало вверх: %+v", names(v.Flexible.Rows))
	}

	if len(v.Cover.Suggestions) != 2 {
		t.Fatalf("подсказки %+v", v.Cover.Suggestions)
	}
	prod := v.Cover.Suggestions[0]
	if prod.Category != "Продукты" || prod.Overspend != 8000 || prod.Uncovered != 0 {
		t.Fatalf("продукты %+v", prod)
	}
	if len(prod.Takes) != 1 || prod.Takes[0].From != "Прочее" || prod.Takes[0].Amount != 8000 {
		t.Fatalf("продукты берут %+v", prod.Takes)
	}
	cafe := v.Cover.Suggestions[1]
	if cafe.Category != "Кафе" || cafe.Overspend != 2000 || cafe.Uncovered != 0 {
		t.Fatalf("кафе %+v", cafe)
	}
	// Прочее was fully used by Продукты, so Кафе takes from Транспорт.
	if len(cafe.Takes) != 1 || cafe.Takes[0].From != "Транспорт" || cafe.Takes[0].Amount != 2000 {
		t.Fatalf("кафе берёт %+v", cafe.Takes)
	}
	if prod.Text != "Продукты: перерасход 8 000 ₽. Возьмите 8 000 ₽ из «Прочее»." {
		t.Fatalf("текст продуктов %q", prod.Text)
	}
	if cafe.Text != "Кафе: перерасход 2 000 ₽. Возьмите 2 000 ₽ из «Транспорт»." {
		t.Fatalf("текст кафе %q", cafe.Text)
	}
}

func TestCoverPartialAndEmpty(t *testing.T) {
	in := prototypeInput(t)
	in.Flexible = []Flex{
		{Name: "Кафе", Plan: ptr(1000), Fact: 6000},
		{Name: "Транспорт", Plan: ptr(2000), Fact: 1000}, // left 1000
	}
	v := Build(in)
	s := v.Cover.Suggestions[0]
	if s.Uncovered != 4000 || len(s.Takes) != 1 || s.Takes[0].Amount != 1000 {
		t.Fatalf("%+v", s)
	}
	want := "Кафе: перерасход 5 000 ₽. Возьмите 1 000 ₽ из «Транспорт». Ещё не хватает 4 000 ₽."
	if s.Text != want {
		t.Fatalf("got %q", s.Text)
	}

	in.Flexible = []Flex{{Name: "Кафе", Plan: ptr(1000), Fact: 6000}}
	v = Build(in)
	if v.Cover.Suggestions[0].Text != "Кафе: перерасход 5 000 ₽. В других статьях запаса нет." {
		t.Fatalf("got %q", v.Cover.Suggestions[0].Text)
	}

	in.Flexible = []Flex{
		{Name: "Кафе", Plan: ptr(5000), Fact: 1000},
		{Name: "Продукты", Plan: ptr(4000), Fact: 1000},
	}
	v = Build(in)
	if len(v.Cover.Suggestions) != 0 || v.Cover.Lines[0] != "Перерасхода нет." {
		t.Fatalf("%+v", v.Cover)
	}
}

func TestCoverSplitsTwoDonors(t *testing.T) {
	in := prototypeInput(t)
	in.Flexible = []Flex{
		{Name: "Кафе", Plan: ptr(1000), Fact: 9000},     // over 8000
		{Name: "Продукты", Plan: ptr(5000), Fact: 1000}, // left 4000
		{Name: "Транспорт", Plan: ptr(4000), Fact: 500}, // left 3500
	}
	v := Build(in)
	s := v.Cover.Suggestions[0]
	if len(s.Takes) != 2 || s.Uncovered != 500 {
		t.Fatalf("%+v", s)
	}
	if s.Takes[0].From != "Продукты" || s.Takes[0].Amount != 4000 {
		t.Fatalf("первый донор %+v", s.Takes[0])
	}
	if s.Takes[1].From != "Транспорт" || s.Takes[1].Amount != 3500 {
		t.Fatalf("второй донор %+v", s.Takes[1])
	}
	want := "Кафе: перерасход 8 000 ₽. Возьмите 4 000 ₽ из «Продукты» и 3 500 ₽ из «Транспорт». Ещё не хватает 500 ₽."
	if s.Text != want {
		t.Fatalf("got %q", s.Text)
	}
}

func names(rows []FlexRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Name
	}
	return out
}

func TestFormatRUB(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{46234, "46 234 ₽"},
		{3082, "3 082 ₽"},
		{3891, "3 891 ₽"},
		{270000, "270 000 ₽"},
		{252526, "252 526 ₽"},
		{0, "0 ₽"},
		{-1000, "-1 000 ₽"},
		{1917, "1 917 ₽"},
	}
	for _, c := range cases {
		if got := FormatRUB(c.n); got != c.want {
			t.Errorf("FormatRUB(%d)=%q want %q", c.n, got, c.want)
		}
	}
}
