package budget

import (
	"testing"
	"time"
)

func TestPeriodSalaryNovember(t *testing.T) {
	p, err := PeriodContaining(time.Date(2026, time.November, 5, 15, 0, 0, 0, time.UTC), 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != KindSalary || p.Days != 15 {
		t.Fatalf("%+v", p)
	}
	if FormatDayMonth(p.Start) != "5.11" || FormatDayMonth(p.End) != "19.11" || FormatDayMonth(p.NextPayday) != "20.11" {
		t.Fatalf("границы %s %s %s", FormatDayMonth(p.Start), FormatDayMonth(p.End), FormatDayMonth(p.NextPayday))
	}
	if p.Month != time.November || p.Year != 2026 {
		t.Fatalf("лист %s %d", p.Month, p.Year)
	}
	y, m := p.PreviousFundingMonth()
	if y != 2026 || m != time.October {
		t.Fatalf("прошлый месяц %s %d", m, y)
	}

	late, err := PeriodContaining(time.Date(2026, time.November, 19, 0, 0, 0, 0, time.UTC), 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	if late.Kind != KindSalary || !late.End.Equal(p.End) {
		t.Fatalf("19.11 не в периоде зарплаты: %+v", late)
	}
}

func TestPeriodAdvanceEdges(t *testing.T) {
	// 4 Nov 2026 is still the October advance, next payday is 5 Nov.
	p, err := PeriodContaining(time.Date(2026, time.November, 4, 23, 0, 0, 0, time.UTC), 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != KindAdvance || p.Month != time.October || p.Year != 2026 {
		t.Fatalf("4.11: %+v", p)
	}
	if FormatDayMonth(p.Start) != "20.10" || FormatDayMonth(p.End) != "4.11" || FormatDayMonth(p.NextPayday) != "5.11" {
		t.Fatalf("границы аванса %s → %s, выплата %s", FormatDayMonth(p.Start), FormatDayMonth(p.End), FormatDayMonth(p.NextPayday))
	}
	// 20 Oct through 31 Oct is 12 days, plus 1-4 Nov is 4, total 16.
	if p.Days != 16 {
		t.Fatalf("дней аванса %d", p.Days)
	}

	start, err := PeriodContaining(time.Date(2026, time.November, 20, 0, 0, 0, 0, time.UTC), 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	if start.Kind != KindAdvance || start.Month != time.November || FormatDayMonth(start.NextPayday) != "5.12" {
		t.Fatalf("20.11: %+v next %s", start, FormatDayMonth(start.NextPayday))
	}
	if FormatDayMonth(start.End) != "4.12" {
		t.Fatalf("конец %s", FormatDayMonth(start.End))
	}

	prev, err := start.Previous(5, 20)
	if err != nil {
		t.Fatal(err)
	}
	if prev.Kind != KindAdvance || prev.Month != time.October || FormatDayMonth(prev.Start) != "20.10" {
		t.Fatalf("предыдущий аванс %+v", prev)
	}
}

func TestPeriodFebruaryAndMarch(t *testing.T) {
	// 1 Mar belongs to the February advance.
	p, err := PeriodContaining(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	if p.Month != time.February || FormatDayMonth(p.Start) != "20.2" || FormatDayMonth(p.End) != "4.3" {
		t.Fatalf("%+v %s %s", p, FormatDayMonth(p.Start), FormatDayMonth(p.End))
	}
	// 2024 is a leap year: 20 Feb-4 Mar includes 29 Feb.
	leap, err := PeriodContaining(time.Date(2024, time.March, 4, 0, 0, 0, 0, time.UTC), 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := PeriodContaining(time.Date(2026, time.March, 4, 0, 0, 0, 0, time.UTC), 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	if leap.Days != plain.Days+1 {
		t.Fatalf("високосный %d обычный %d", leap.Days, plain.Days)
	}
}

func TestPeriodRejectsBadDays(t *testing.T) {
	if _, err := PeriodContaining(time.Now(), 20, 5); err == nil {
		t.Fatal("зарплата позже аванса должна быть ошибкой")
	}
}

func TestTopMetaPrototype(t *testing.T) {
	p, err := PeriodOf(2026, time.November, KindSalary, 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	today := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	usePrev, column, note := TopMeta(today, p, false)
	if !usePrev || column != "Топ-5 октября" {
		t.Fatalf("usePrev %v column %q", usePrev, column)
	}
	if note != "Ноябрьский период ещё не начался, поэтому показан октябрь." {
		t.Fatalf("note %q", note)
	}
}
