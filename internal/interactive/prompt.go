package interactive

import (
	"sort"

	"github.com/charmbracelet/huh"
)

// Select shows a single-choice menu and returns the index of the chosen item.
// With startInSearchMode, typing narrows the list right away.
func Select(label string, options []string, size int, startInSearchMode bool) (int, error) {
	opts := make([]huh.Option[int], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, i)
	}
	var chosen int
	field := huh.NewSelect[int]().
		Title(label).
		Options(opts...).
		Filtering(startInSearchMode).
		Value(&chosen)
	if len(options) > size {
		field.Height(size + 2) // room for the title and the filter line
	}
	if err := field.Run(); err != nil {
		return 0, err
	}
	return chosen, nil
}

// Confirm asks a yes/no question. defaultNo puts the cursor on No, so pressing
// Enter declines.
func Confirm(label string, defaultNo bool) (bool, error) {
	answer := !defaultNo
	err := huh.NewConfirm().
		Title(label).
		Affirmative("Yes").
		Negative("No").
		Value(&answer).
		Run()
	if err != nil {
		return false, err
	}
	return answer, nil
}

// MultiSelect lets the user check any number of items, sorted, with filtering
// ("/" to search), and returns the checked ones in sorted order. Checking none
// returns nil.
func MultiSelect(label string, items []string, size int) ([]string, error) {
	if len(items) == 0 {
		return nil, nil
	}
	sorted := append([]string{}, items...)
	sort.Strings(sorted)

	var selected []string
	field := huh.NewMultiSelect[string]().
		Title(label).
		Description("space to check, / to search, enter to confirm").
		Options(huh.NewOptions(sorted...)...).
		Filterable(true).
		Value(&selected)
	if len(sorted) > size {
		field.Height(size + 3) // room for the title, description and filter line
	}
	if err := field.Run(); err != nil {
		return nil, err
	}
	return selected, nil
}
