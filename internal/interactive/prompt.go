package interactive

import (
	"sort"

	"github.com/manifoldco/promptui"
)

// Select shows a single-choice menu and returns the index of the chosen item.
func Select(label string, options []string, size int, startInSearchMode bool) (int, error) {
	sel := promptui.Select{
		Label:             label,
		Items:             options,
		Size:              min(len(options), size),
		StartInSearchMode: startInSearchMode,
	}
	i, _, err := sel.Run()
	if err != nil {
		return 0, err
	}
	return i, nil
}

// Confirm asks a yes/no question using promptui's confirm mode.
func Confirm(label string, defaultNo bool) (bool, error) {
	p := promptui.Prompt{
		Label:     label,
		IsConfirm: true,
	}
	if defaultNo {
		p.Default = "n"
	} else {
		p.Default = "y"
	}
	_, err := p.Run()
	if err == nil {
		return true, nil
	}
	if err == promptui.ErrAbort {
		return false, nil
	}
	return false, err
}

// YesNo is a non-aborting confirmation prompt implemented as a select.
// This avoids promptui's confirm-mode abort behavior (which can look like an
// error) when the user answers "no".
func YesNo(label string, defaultNo bool) (bool, error) {
	items := []string{"No", "Yes"}
	start := 0
	if !defaultNo {
		start = 1
	}
	sel := promptui.Select{
		Label:     label,
		Items:     items,
		Size:      2,
		CursorPos: start,
	}
	i, _, err := sel.Run()
	if err != nil {
		return false, err
	}
	return i == 1, nil
}

// MultiSelectByDone is a dependency-free "multi-select" UX:
// users repeatedly pick one item from the list until they choose [Done].
func MultiSelectByDone(label string, items []string, size int) ([]string, error) {
	if len(items) == 0 {
		return nil, nil
	}

	remaining := append([]string{}, items...)
	sort.Strings(remaining)

	var selected []string
	for len(remaining) > 0 {
		menu := append([]string{"[Done]"}, remaining...)
		i, err := Select(label, menu, size, true)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			break
		}
		chosen := menu[i]
		selected = append(selected, chosen)

		next := make([]string, 0, len(remaining)-1)
		for _, it := range remaining {
			if it != chosen {
				next = append(next, it)
			}
		}
		remaining = next
	}
	return selected, nil
}
