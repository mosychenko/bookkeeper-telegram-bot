package sheet

import (
	"fmt"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
)

// FixtureFile is the approved November prototype. It is what local dev and
// the tests render. It is not the live sheet.
type FixtureFile struct {
	AsOf           string `yaml:"as_of"`
	Year           int    `yaml:"year"`
	Month          int    `yaml:"month"`
	Period         string `yaml:"period"`
	PlannedIncome  int64  `yaml:"planned_income"`
	LastIncome     int64  `yaml:"last_income"`
	LastFlexPerDay int64  `yaml:"last_flex_per_day"`
	Paid           map[string]struct {
		Note string `yaml:"note"`
	} `yaml:"paid"`
	Flexible []struct {
		Category  string `yaml:"category"`
		Fact      int64  `yaml:"fact"`
		LastMonth int64  `yaml:"last_month"`
		Plan      *int64 `yaml:"plan"`
	} `yaml:"flexible"`
	Top []struct {
		Name   string `yaml:"name"`
		Amount int64  `yaml:"amount"`
	} `yaml:"top"`
}

func LoadFixture(budgetPath, fixturePath string) (budget.Input, error) {
	bf, err := LoadBudget(budgetPath)
	if err != nil {
		return budget.Input{}, err
	}
	var fx FixtureFile
	if err := readYAML(fixturePath, &fx); err != nil {
		return budget.Input{}, err
	}
	asOf, err := time.ParseInLocation("2006-01-02", fx.AsOf, time.UTC)
	if err != nil {
		return budget.Input{}, fmt.Errorf("fixture.as_of: %w", err)
	}
	if fx.Month < 1 || fx.Month > 12 || fx.Year < 2000 {
		return budget.Input{}, fmt.Errorf("fixture: месяц и год")
	}
	period, err := budget.PeriodOf(fx.Year, time.Month(fx.Month), budget.Kind(fx.Period), bf.Payday.SalaryDay, bf.Payday.AdvanceDay)
	if err != nil {
		return budget.Input{}, err
	}

	var mandatory []budget.Payment
	for _, item := range bf.ForPeriod(period.Kind) {
		if item.Optional && item.AmountDefault == 0 {
			continue
		}
		p := budget.Payment{
			Name:   item.Name,
			Amount: item.AmountDefault,
			Due:    item.DueLabel(period.Month),
		}
		if note, ok := fx.Paid[item.ID]; ok {
			p.Paid = true
			p.PaidNote = note.Note
		}
		mandatory = append(mandatory, p)
	}

	flexible := make([]budget.Flex, 0, len(fx.Flexible))
	for _, row := range fx.Flexible {
		flexible = append(flexible, budget.Flex{
			Name:      row.Category,
			Plan:      row.Plan,
			Fact:      row.Fact,
			LastMonth: row.LastMonth,
		})
	}
	top := make([]budget.Spent, 0, len(fx.Top))
	for _, row := range fx.Top {
		top = append(top, budget.Spent{Name: row.Name, Amount: row.Amount})
	}

	return budget.Input{
		Now:             asOf,
		AsOf:            asOf,
		Period:          period,
		Demo:            true,
		Planned:         fx.PlannedIncome,
		LastIncome:      fx.LastIncome,
		LastIncomeKnown: true,
		LastFlexPerDay:  fx.LastFlexPerDay,
		PaceKnown:       true,
		Mandatory:       mandatory,
		Flexible:        flexible,
		Top:             top,
		HasPeriodSpend:  false,
	}, nil
}
