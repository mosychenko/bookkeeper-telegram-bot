package sheet

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mosychenko/bookkeeper-telegram-bot/internal/budget"
	"gopkg.in/yaml.v3"
)

// BudgetFile is the family config: which categories are mandatory, when they
// leave the account, and which are flexible. Amounts in the sheet win;
// amount_default is only a fallback (and the fixture's source of truth).
type BudgetFile struct {
	Payday struct {
		SalaryDay  int `yaml:"salary_day"`
		AdvanceDay int `yaml:"advance_day"`
	} `yaml:"payday"`
	Mandatory []MandatoryItem `yaml:"mandatory"`
	Flexible  []FlexibleItem  `yaml:"flexible"`
	Income    struct {
		Salary  int64 `yaml:"salary"`
		Advance int64 `yaml:"advance"`
	} `yaml:"income_defaults"`
	PaidStatusValues []string `yaml:"paid_status_values"`
}

type MandatoryItem struct {
	ID            string `yaml:"id"`
	Name          string `yaml:"name"`
	Category      string `yaml:"category"`
	Period        string `yaml:"period"` // salary | advance
	AmountDefault int64  `yaml:"amount_default"`
	Optional      bool   `yaml:"optional"`
	Due           *struct {
		Day         int  `yaml:"day"`
		Approximate bool `yaml:"approximate"`
	} `yaml:"due"`
	DueDays []int  `yaml:"due_days"`
	DueText string `yaml:"due_text"`
}

type FlexibleItem struct {
	Category string `yaml:"category"`
}

func LoadBudget(path string) (BudgetFile, error) {
	var f BudgetFile
	if err := readYAML(path, &f); err != nil {
		return f, err
	}
	if err := f.Validate(); err != nil {
		return f, err
	}
	return f, nil
}

func (f BudgetFile) Validate() error {
	if f.Payday.SalaryDay < 1 || f.Payday.AdvanceDay < 1 || f.Payday.SalaryDay >= f.Payday.AdvanceDay || f.Payday.AdvanceDay > 28 {
		return fmt.Errorf("payday: зарплата %d, аванс %d", f.Payday.SalaryDay, f.Payday.AdvanceDay)
	}
	seen := map[string]bool{}
	for _, m := range f.Mandatory {
		if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Category) == "" {
			return fmt.Errorf("у обязательного платежа нет имени или категории")
		}
		if m.Period != string(budget.KindSalary) && m.Period != string(budget.KindAdvance) {
			return fmt.Errorf("%s: period должен быть salary или advance", m.Name)
		}
		if m.ID != "" && seen[m.ID] {
			return fmt.Errorf("повтор id %s", m.ID)
		}
		seen[m.ID] = true
	}
	flex := map[string]bool{}
	for _, c := range f.Flexible {
		n := norm(c.Category)
		if n == "" {
			return fmt.Errorf("пустая гибкая категория")
		}
		if flex[n] {
			return fmt.Errorf("категория %s указана дважды", c.Category)
		}
		flex[n] = true
	}
	return nil
}

func (f BudgetFile) ForPeriod(kind budget.Kind) []MandatoryItem {
	var out []MandatoryItem
	for _, m := range f.Mandatory {
		if m.Period == string(kind) {
			out = append(out, m)
		}
	}
	return out
}

func (f BudgetFile) FlexibleSet() map[string]FlexibleItem {
	out := make(map[string]FlexibleItem, len(f.Flexible))
	for _, c := range f.Flexible {
		out[norm(c.Category)] = c
	}
	return out
}

func (f BudgetFile) IsFlexible(category string) bool {
	_, ok := f.FlexibleSet()[norm(category)]
	return ok
}

func (m MandatoryItem) DueLabel(month time.Month) string {
	if strings.TrimSpace(m.DueText) != "" {
		return m.DueText
	}
	if len(m.DueDays) > 0 {
		parts := make([]string, len(m.DueDays))
		for i, d := range m.DueDays {
			parts[i] = strconv.Itoa(d)
		}
		return joinDays(parts) + "." + strconv.Itoa(int(month))
	}
	if m.Due != nil && m.Due.Day > 0 {
		prefix := ""
		if m.Due.Approximate {
			prefix = "~"
		}
		return prefix + strconv.Itoa(m.Due.Day) + "." + strconv.Itoa(int(month))
	}
	return "дата ?"
}

func (f BudgetFile) paid(status string) bool {
	status = strings.TrimSpace(status)
	if status == "" {
		return false
	}
	for _, v := range f.PaidStatusValues {
		if strings.EqualFold(status, strings.TrimSpace(v)) {
			return true
		}
	}
	return false
}

func joinDays(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " и " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " и " + parts[len(parts)-1]
	}
}

func readYAML(path string, dest any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("не прочитать %s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, dest); err != nil {
		return fmt.Errorf("разобрать %s: %w", path, err)
	}
	return nil
}

func norm(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
