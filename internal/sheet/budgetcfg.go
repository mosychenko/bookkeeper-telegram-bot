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
// leave the account, and which are flexible. The sheet does not mark that
// split and has no payment dates, so both live here. Amounts come from the
// period column of the monthly tab. amount_default is not applied to a
// blank cell: a blank cell means the category is not in that period.
type BudgetFile struct {
	Payday struct {
		SalaryDay  int `yaml:"salary_day"`
		AdvanceDay int `yaml:"advance_day"`
	} `yaml:"payday"`
	Mandatory []MandatoryItem `yaml:"mandatory"`
	Flexible  []FlexibleItem  `yaml:"flexible"`
	Rare      []FlexibleItem  `yaml:"rare"`
	Income    struct {
		Salary  int64 `yaml:"salary"`
		Advance int64 `yaml:"advance"`
	} `yaml:"income_defaults"`
	// SalaryIncomeCategories are the only inflows that count as salary.
	// Anything else, including «Прочие поступления» and loans, does not.
	SalaryIncomeCategories []string `yaml:"salary_income_categories"`
	// ExcludedIncomeCategories never count as free money. Their full
	// amount is added to the копилка (rent, once the sheet has a name).
	ExcludedIncomeCategories []string `yaml:"excluded_income_categories"`
	// SavingsRate is the share of salary income that goes to the копилка.
	SavingsRate      float64  `yaml:"savings_rate"`
	PaidStatusValues []string `yaml:"paid_status_values"`
}

type MandatoryItem struct {
	ID             string   `yaml:"id"`
	Name           string   `yaml:"name"`
	Category       string   `yaml:"category"`
	OperationsName string   `yaml:"operations_name"`
	Period         string   `yaml:"period"` // legacy single period; periods wins when set
	Periods        []string `yaml:"periods"`
	AmountDefault  int64    `yaml:"amount_default"`
	// SalaryIfBlank and AdvanceIfBlank fill a period only when that
	// column is empty. Zero means «leave the blank cell blank».
	SalaryIfBlank  int64   `yaml:"salary_if_blank"`
	AdvanceIfBlank int64   `yaml:"advance_if_blank"`
	Optional       bool    `yaml:"optional"`
	Due            *DueDay `yaml:"due"`
	DueSalary      *DueDay `yaml:"due_salary"`
	DueAdvance     *DueDay `yaml:"due_advance"`
	DueDays        []int   `yaml:"due_days"`
	DueRange       []int   `yaml:"due_range"`
	DueText        string  `yaml:"due_text"`
}

// DueDay is one payment day. Approximate prints a tilde.
type DueDay struct {
	Day         int  `yaml:"day"`
	Approximate bool `yaml:"approximate"`
}

type FlexibleItem struct {
	Category       string `yaml:"category"`
	OperationsName string `yaml:"operations_name"`
	Rare           bool   `yaml:"rare"`
}

func (m MandatoryItem) opsCategory() string {
	if strings.TrimSpace(m.OperationsName) != "" {
		return m.OperationsName
	}
	return m.Category
}

func (item FlexibleItem) opsCategory() string {
	if strings.TrimSpace(item.OperationsName) != "" {
		return item.OperationsName
	}
	return item.Category
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
		if p := strings.TrimSpace(m.Period); p != "" && p != string(budget.KindSalary) && p != string(budget.KindAdvance) {
			return fmt.Errorf("%s: period должен быть salary или advance", m.Name)
		}
		for _, p := range m.Periods {
			p = strings.TrimSpace(p)
			if p != string(budget.KindSalary) && p != string(budget.KindAdvance) {
				return fmt.Errorf("%s: periods содержит %q", m.Name, p)
			}
		}
		if m.SalaryIfBlank < 0 || m.AdvanceIfBlank < 0 {
			return fmt.Errorf("%s: запасная сумма отрицательная", m.Name)
		}
		if len(m.DueRange) != 0 && (len(m.DueRange) != 2 || m.DueRange[0] < 1 || m.DueRange[1] > 31 || m.DueRange[0] > m.DueRange[1]) {
			return fmt.Errorf("%s: due_range", m.Name)
		}
		if m.ID != "" && seen[m.ID] {
			return fmt.Errorf("повтор id %s", m.ID)
		}
		seen[m.ID] = true
	}
	flex := map[string]bool{}
	if err := addFlex(flex, f.Flexible); err != nil {
		return err
	}
	if err := addFlex(flex, f.Rare); err != nil {
		return err
	}
	for _, m := range f.Mandatory {
		if flex[norm(m.Category)] {
			return fmt.Errorf("%s указана и как обязательная, и как гибкая", m.Name)
		}
	}
	if err := f.validateIncome(); err != nil {
		return err
	}
	return nil
}

func (f BudgetFile) validateIncome() error {
	if !f.hasSavings() {
		return nil
	}
	if f.SavingsRate <= 0 || f.SavingsRate > 1 {
		return fmt.Errorf("savings_rate должен быть от 0 до 1, сейчас %v", f.SavingsRate)
	}
	if len(f.SalaryIncomeCategories) == 0 {
		return fmt.Errorf("salary_income_categories пуст")
	}
	seen := map[string]bool{}
	for _, c := range f.SalaryIncomeCategories {
		n := norm(c)
		if n == "" {
			return fmt.Errorf("пустая категория зарплаты")
		}
		if seen[n] {
			return fmt.Errorf("категория зарплаты %s указана дважды", c)
		}
		seen[n] = true
	}
	for _, c := range f.ExcludedIncomeCategories {
		n := norm(c)
		if n == "" {
			return fmt.Errorf("пустая исключённая категория дохода")
		}
		if seen[n] {
			return fmt.Errorf("%s указана и как зарплата, и как исключённый доход", c)
		}
		seen[n] = true
	}
	return nil
}

func (f BudgetFile) hasSavings() bool {
	for _, m := range f.Mandatory {
		if m.ID == "savings" {
			return true
		}
	}
	return false
}

func (f BudgetFile) listed(categories []string, name string) bool {
	n := norm(name)
	if n == "" {
		return false
	}
	for _, c := range categories {
		if norm(c) == n {
			return true
		}
	}
	return false
}

func addFlex(seen map[string]bool, list []FlexibleItem) error {
	for _, c := range list {
		n := norm(c.Category)
		if n == "" {
			return fmt.Errorf("пустая гибкая категория")
		}
		if seen[n] {
			return fmt.Errorf("категория %s указана дважды", c.Category)
		}
		seen[n] = true
	}
	return nil
}

// flexItems is every category that behaves as flexible, including rare ones.
func (f BudgetFile) flexItems() []FlexibleItem {
	out := make([]FlexibleItem, 0, len(f.Flexible)+len(f.Rare))
	out = append(out, f.Flexible...)
	for _, item := range f.Rare {
		item.Rare = true
		out = append(out, item)
	}
	return out
}

func (m MandatoryItem) allows(kind budget.Kind) bool {
	if len(m.Periods) == 0 {
		if p := strings.TrimSpace(m.Period); p != "" {
			return p == string(kind)
		}
		return true
	}
	for _, p := range m.Periods {
		if strings.TrimSpace(p) == string(kind) {
			return true
		}
	}
	return false
}

func (m MandatoryItem) blankFill(kind budget.Kind) int64 {
	if kind == budget.KindAdvance {
		return m.AdvanceIfBlank
	}
	return m.SalaryIfBlank
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
	items := f.flexItems()
	out := make(map[string]FlexibleItem, len(items))
	for _, c := range items {
		out[norm(c.Category)] = c
	}
	return out
}

func (f BudgetFile) IsFlexible(category string) bool {
	_, ok := f.FlexibleSet()[norm(category)]
	return ok
}

func (m MandatoryItem) DueLabel(month time.Month, kind budget.Kind) string {
	if label, ok := m.periodDue(kind, month); ok {
		return label
	}
	if strings.TrimSpace(m.DueText) != "" {
		return m.DueText
	}
	if len(m.DueRange) == 2 {
		return strconv.Itoa(m.DueRange[0]) + "–" + strconv.Itoa(m.DueRange[1]) + "." + strconv.Itoa(int(month))
	}
	if len(m.DueDays) > 0 {
		parts := make([]string, len(m.DueDays))
		for i, d := range m.DueDays {
			parts[i] = strconv.Itoa(d)
		}
		return joinDays(parts) + "." + strconv.Itoa(int(month))
	}
	if m.Due != nil && m.Due.Day > 0 {
		return formatDay(m.Due.Day, m.Due.Approximate, month)
	}
	return "дата ?"
}

func (m MandatoryItem) periodDue(kind budget.Kind, month time.Month) (string, bool) {
	var day *DueDay
	switch kind {
	case budget.KindSalary:
		day = m.DueSalary
	case budget.KindAdvance:
		day = m.DueAdvance
	}
	if day != nil && day.Day > 0 {
		return formatDay(day.Day, day.Approximate, month), true
	}
	return "", false
}

func formatDay(day int, approx bool, month time.Month) string {
	prefix := ""
	if approx {
		prefix = "~"
	}
	return prefix + strconv.Itoa(day) + "." + strconv.Itoa(int(month))
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
