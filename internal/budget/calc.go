package budget

import (
	"cmp"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

// Payment is one mandatory obligation of the period.
// The displayed total always includes Amount. Paid means the money has
// already left the account, so a known balance is not reduced by it.
// CoveredByBalance is the part of Amount the sheet already removed from
// the balance (the whole piggy-bank line: 10% of salary and rent).
type Payment struct {
	Name             string `json:"name"`
	Amount           int64  `json:"amount"`
	Due              string `json:"due"`
	Paid             bool   `json:"paid"`
	PaidNote         string `json:"paidNote,omitempty"`
	CoveredByBalance int64  `json:"coveredByBalance,omitempty"`
}

// Flex is one flexible category. A nil Plan means the limit is not set
// ("не задан") and the category cannot be overspent.
type Flex struct {
	Name      string `json:"name"`
	Plan      *int64 `json:"plan"`
	Fact      int64  `json:"fact"`
	LastMonth int64  `json:"lastMonth"`
}

// Spent is one row of "Куда ушли деньги".
type Spent struct {
	Name   string `json:"name"`
	Amount int64  `json:"amount"`
}

// Input is everything the screen needs. Fixture and the sheet assembler
// both produce this; Build does not read the table itself.
type Input struct {
	Now    time.Time
	AsOf   time.Time
	Period Period
	Demo   bool
	// Balance is the account total already net of the whole piggy bank.
	// It is used only when BalanceKnown. Otherwise free money falls back
	// to Planned, and the caller explains that in Warnings.
	Balance      int64
	BalanceKnown bool
	// BalanceAsOf is the sheet sync date. Zero means "use Now, then AsOf".
	BalanceAsOf time.Time
	Planned     int64
	// LastIncome is the same kind of inflow last month (salary or advance).
	LastIncome      int64
	LastIncomeKnown bool
	LastFlexPerDay  int64
	PaceKnown       bool
	Mandatory       []Payment
	Flexible        []Flex
	Top             []Spent
	// HasPeriodSpend is false when the period already started but there are
	// no expenses in it yet. The screen then shows the previous month and
	// says so. Ignored when today is still before the period.
	HasPeriodSpend bool
	Warnings       []string
}

type View struct {
	Title         string        `json:"title"`
	AsOf          string        `json:"asOf"`
	PeriodLabel   string        `json:"periodLabel"`
	Days          int           `json:"days"`
	Question      string        `json:"question"`
	Status        string        `json:"status"`
	StatusText    string        `json:"statusText"`
	Free          int64         `json:"free"`
	PerDay        int64         `json:"perDay"`
	FreeCaption   string        `json:"freeCaption"`
	PerDayCaption string        `json:"perDayCaption"`
	RiskText      string        `json:"riskText"`
	LastPace      int64         `json:"lastPace"`
	PaceKnown     bool          `json:"paceKnown"`
	Mandatory     MandatoryView `json:"mandatory"`
	Flexible      FlexibleView  `json:"flexible"`
	Cover         CoverView     `json:"cover"`
	Top           TopView       `json:"top"`
	Warnings      []string      `json:"warnings"`
	SourceNote    string        `json:"sourceNote"`
}

type MandatoryView struct {
	Heading   string    `json:"heading"`
	Caption   string    `json:"caption"`
	Rows      []Payment `json:"rows"`
	Total     int64     `json:"total"`
	Covered   bool      `json:"covered"`
	Shortfall int64     `json:"shortfall"`
	TotalText string    `json:"totalText"`
}

type FlexRow struct {
	Name      string `json:"name"`
	Plan      *int64 `json:"plan"`
	Fact      int64  `json:"fact"`
	Left      *int64 `json:"left"`
	LastMonth int64  `json:"lastMonth"`
	Overspent bool   `json:"overspent"`
}

type FlexibleView struct {
	Heading   string    `json:"heading"`
	Caption   string    `json:"caption"`
	PrevMonth string    `json:"prevMonth"`
	Rows      []FlexRow `json:"rows"`
}

type Take struct {
	From   string `json:"from"`
	Amount int64  `json:"amount"`
}

type Suggestion struct {
	Category  string `json:"category"`
	Overspend int64  `json:"overspend"`
	Takes     []Take `json:"takes"`
	Uncovered int64  `json:"uncovered"`
	Text      string `json:"text"`
}

type CoverView struct {
	Heading     string       `json:"heading"`
	Lines       []string     `json:"lines"`
	Suggestions []Suggestion `json:"suggestions"`
}

type TopView struct {
	Heading string  `json:"heading"`
	Column  string  `json:"column"`
	Note    string  `json:"note"`
	Rows    []Spent `json:"rows"`
}

// Build answers "will the money last until the next payday?".
//
// With a known balance, free money is that balance minus mandatory
// payments that are still ahead. Paid lines have already left the
// account. CoveredByBalance (the piggy bank) is already out of the
// balance, so it is not taken off again. "Per day" divides by the days
// from the balance date through the day before the next payday.
//
// Without a balance, free money is planned income minus every mandatory
// amount, paid or not, over the whole period. That is the prototype.
//
// The status compares the exact free/day ratio with last period's
// flexible pace. Both numbers are rubles per day, so a shorter remainder
// does not change the meaning of «Впритык».
//
// Mandatory.Total and the risk line always use the full mandatory sum.
// The risk "per day" stays on the full period: it is a what-if about
// the period's income, not about today's balance.
func Build(in Input) View {
	rows := append([]Payment(nil), in.Mandatory...)
	if rows == nil {
		rows = []Payment{}
	}
	var mandatoryTotal int64
	var stillToPay int64
	for _, p := range rows {
		mandatoryTotal += p.Amount
		if p.Paid {
			continue
		}
		rest := p.Amount - p.CoveredByBalance
		if rest > 0 {
			stillToPay += rest
		}
	}
	// Unpaid first, then paid. Inside each group, larger amounts come first.
	// Equal amounts keep the order from the config.
	slices.SortStableFunc(rows, func(a, b Payment) int {
		if a.Paid != b.Paid {
			if a.Paid {
				return 1
			}
			return -1
		}
		return cmp.Compare(b.Amount, a.Amount)
	})

	days := in.Period.Days
	free := in.Planned - mandatoryTotal
	if in.BalanceKnown {
		free = in.Balance - stillToPay
		days = daysThroughPayday(balanceClock(in), in.Period.NextPayday)
	}
	if days < 1 {
		days = 1
	}
	exactPerDay := float64(free) / float64(days)
	perDay := RoundDiv(free, int64(days))

	status, statusText := statusOf(free, exactPerDay, in.PaceKnown, in.LastFlexPerDay)

	prevYear, prevMonth := in.Period.PreviousFundingMonth()
	_ = prevYear
	riskDays := in.Period.Days
	if riskDays < 1 {
		riskDays = 1
	}
	riskFree := in.LastIncome - mandatoryTotal
	riskPerDay := RoundDiv(riskFree, int64(riskDays))

	flexRows := flexibleRows(in.Flexible)
	suggestions := cover(flexRows)
	lines := coverLines(suggestions, in.Period.Month, anyPlan(in.Flexible))

	today := in.Now
	if today.IsZero() {
		today = in.AsOf
	}
	_, column, note := TopMeta(today, in.Period, in.HasPeriodSpend)

	warnings := in.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	if suggestions == nil {
		suggestions = []Suggestion{}
	}
	topRows := in.Top
	if topRows == nil {
		topRows = []Spent{}
	}

	shortfall := int64(0)
	totalText := "денег хватает"
	if free < 0 {
		shortfall = -free
		totalText = "не хватает " + FormatRUB(shortfall)
	}

	sourceNote := "Таблица «Бюджет». Цифры обновляются раз в несколько минут."
	if in.Demo {
		sourceNote = "Это пример ноября из прототипа, не живая таблица."
	}

	return View{
		Title:         "Деньги до зарплаты",
		AsOf:          FormatDayMonthPadded(in.AsOf),
		PeriodLabel:   in.Period.Label(),
		Days:          days,
		Question:      "Хватит до " + FormatDayMonth(in.Period.NextPayday) + "?",
		Status:        status,
		StatusText:    statusText,
		Free:          free,
		PerDay:        perDay,
		FreeCaption:   "свободно после обязательных",
		PerDayCaption: fmt.Sprintf("можно тратить в день, %d %s", days, daysWord(days)),
		RiskText:      riskText(in, prevMonth, riskFree, riskPerDay),
		LastPace:      in.LastFlexPerDay,
		PaceKnown:     in.PaceKnown,
		Mandatory: MandatoryView{
			Heading:   "Обязательные до " + FormatDayMonth(in.Period.NextPayday),
			Caption:   "Даты с «~» — как списывалось в " + MonthPrepositional(prevMonth),
			Rows:      rows,
			Total:     mandatoryTotal,
			Covered:   free >= 0,
			Shortfall: shortfall,
			TotalText: totalText,
		},
		Flexible: FlexibleView{
			Heading:   "Гибкие траты",
			Caption:   flexCaption(in.Period.Month, anyPlan(in.Flexible)),
			PrevMonth: MonthNominative(prevMonth),
			Rows:      flexRows,
		},
		Cover: CoverView{
			Heading:     "Откуда взять",
			Lines:       lines,
			Suggestions: suggestions,
		},
		Top: TopView{
			Heading: "Куда ушли деньги",
			Column:  column,
			Note:    note,
			Rows:    topRows,
		},
		Warnings:   warnings,
		SourceNote: sourceNote,
	}
}

// balanceClock is the day the balance describes: the sheet sync date,
// otherwise today, otherwise the data date.
func balanceClock(in Input) time.Time {
	if !in.BalanceAsOf.IsZero() {
		return DateOnly(in.BalanceAsOf)
	}
	if !in.Now.IsZero() {
		return DateOnly(in.Now)
	}
	return DateOnly(in.AsOf)
}

// daysThroughPayday counts the calendar days from from through the day
// before nextPayday. That is the same inclusive span as Period.Days.
func daysThroughPayday(from, nextPayday time.Time) int {
	end := DateOnly(nextPayday).AddDate(0, 0, -1)
	return inclusiveDays(DateOnly(from), end)
}

func statusOf(free int64, exactPerDay float64, paceKnown bool, pace int64) (string, string) {
	switch {
	case free < 0:
		return "no", "Нет"
	case paceKnown && exactPerDay < float64(pace):
		return "tight", "Впритык"
	default:
		return "ok", "Да"
	}
}

func riskText(in Input, prev time.Month, riskFree, riskPerDay int64) string {
	var b strings.Builder
	where := MonthPrepositional(prev)
	if in.PaceKnown {
		fmt.Fprintf(&b, "В %s на гибкие траты уходило %s в день. ", where, FormatRUB(in.LastFlexPerDay))
	}
	if in.LastIncomeKnown {
		fmt.Fprintf(&b, "Если %s придёт как в %s (%s), а не %s по плану, свободно %s, это %s в день.",
			in.Period.IncomeWord(), where, FormatRUB(in.LastIncome), FormatRUB(in.Planned), FormatRUB(riskFree), FormatRUB(riskPerDay))
	}
	return strings.TrimSpace(b.String())
}

func flexCaption(month time.Month, plansSet bool) string {
	if !plansSet {
		return "В плане " + MonthGenitive(month) + " лимиты не заданы: всё идёт из свободного остатка"
	}
	return "Сначала статьи выше лимита. Прошлый месяц — для сравнения."
}

func anyPlan(rows []Flex) bool {
	for _, r := range rows {
		if r.Plan != nil {
			return true
		}
	}
	return false
}

func overspendOf(f Flex) int64 {
	if f.Plan == nil {
		return 0
	}
	d := f.Fact - *f.Plan
	if d < 0 {
		return 0
	}
	return d
}

func flexibleRows(in []Flex) []FlexRow {
	rows := make([]FlexRow, len(in))
	for i, f := range in {
		var left *int64
		if f.Plan != nil {
			v := *f.Plan - f.Fact
			left = &v
		}
		rows[i] = FlexRow{
			Name:      f.Name,
			Plan:      f.Plan,
			Fact:      f.Fact,
			Left:      left,
			LastMonth: f.LastMonth,
			Overspent: overspendOf(f) > 0,
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ai := overAmount(rows[i])
		aj := overAmount(rows[j])
		if ai > 0 && aj == 0 {
			return true
		}
		if aj > 0 && ai == 0 {
			return false
		}
		if ai > 0 && aj > 0 && ai != aj {
			return ai > aj
		}
		return false
	})
	return rows
}

func overAmount(r FlexRow) int64 {
	if r.Plan == nil || r.Fact <= *r.Plan {
		return 0
	}
	return r.Fact - *r.Plan
}

func surplus(r FlexRow) int64 {
	if r.Plan == nil {
		return 0
	}
	s := *r.Plan - r.Fact
	if s < 0 {
		return 0
	}
	return s
}

// cover suggests where to take money when a flexible category is over its limit.
// Larger gaps are covered first. A donor's remaining budget is spent only once.
func cover(rows []FlexRow) []Suggestion {
	left := make(map[string]int64, len(rows))
	for _, r := range rows {
		left[r.Name] = surplus(r)
	}
	var out []Suggestion
	for _, r := range rows {
		need := overAmount(r)
		if need == 0 {
			continue
		}
		type donor struct {
			name string
			left int64
		}
		var donors []donor
		for _, other := range rows {
			if other.Name == r.Name || left[other.Name] <= 0 {
				continue
			}
			donors = append(donors, donor{other.Name, left[other.Name]})
		}
		sort.Slice(donors, func(i, j int) bool {
			if donors[i].left != donors[j].left {
				return donors[i].left > donors[j].left
			}
			return donors[i].name < donors[j].name
		})
		s := Suggestion{Category: r.Name, Overspend: need}
		for _, d := range donors {
			if need == 0 {
				break
			}
			take := d.left
			if take > need {
				take = need
			}
			if take <= 0 {
				continue
			}
			left[d.name] -= take
			need -= take
			s.Takes = append(s.Takes, Take{From: d.name, Amount: take})
		}
		if s.Takes == nil {
			s.Takes = []Take{}
		}
		s.Uncovered = need
		s.Text = suggestionText(s)
		out = append(out, s)
	}
	return out
}

func suggestionText(s Suggestion) string {
	head := fmt.Sprintf("%s: перерасход %s.", s.Category, FormatRUB(s.Overspend))
	if len(s.Takes) == 0 {
		return head + " В других статьях запаса нет."
	}
	parts := make([]string, len(s.Takes))
	for i, t := range s.Takes {
		parts[i] = fmt.Sprintf("%s из «%s»", FormatRUB(t.Amount), t.From)
	}
	text := head + " Возьмите " + joinRu(parts) + "."
	if s.Uncovered > 0 {
		text += " Ещё не хватает " + FormatRUB(s.Uncovered) + "."
	}
	return text
}

func joinRu(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " и " + parts[len(parts)-1]
	}
}

func coverLines(suggestions []Suggestion, month time.Month, plansSet bool) []string {
	if len(suggestions) == 0 {
		if !plansSet {
			return []string{fmt.Sprintf("Перерасхода пока нет. Чтобы подсказка заработала, нужны лимиты по гибким статьям в листе «%s».", MonthNominative(month))}
		}
		return []string{"Перерасхода нет."}
	}
	lines := make([]string, len(suggestions))
	for i, s := range suggestions {
		lines[i] = s.Text
	}
	return lines
}

// TopMeta describes the collapsed "Куда ушли деньги" block.
// When today is before the period starts, the copy matches the prototype:
// "Ноябрьский период ещё не начался, поэтому показан октябрь."
func TopMeta(today time.Time, period Period, hasCurrentSpend bool) (usePrevious bool, column, note string) {
	_, prevMonth := period.PreviousFundingMonth()
	prevName := strings.ToLower(MonthNominative(prevMonth))
	if DateOnly(today).Before(period.Start) {
		return true,
			"Топ-5 " + MonthGenitive(prevMonth),
			fmt.Sprintf("%s период ещё не начался, поэтому показан %s.", MonthAdjective(period.Month), prevName)
	}
	if !hasCurrentSpend {
		return true,
			"Топ-5 " + MonthGenitive(prevMonth),
			fmt.Sprintf("В этом периоде трат пока нет, поэтому показан %s.", prevName)
	}
	return false, currentColumn(period), ""
}

func currentColumn(period Period) string {
	if period.Start.Month() != period.End.Month() {
		return "Топ-5 периода"
	}
	return "Топ-5 " + MonthGenitive(period.Month)
}
