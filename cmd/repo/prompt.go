package repo

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/manifoldco/promptui"
)

func isInteractiveTTY() (bool, error) {
	in, err := os.Stdin.Stat()
	if err != nil {
		return false, err
	}
	return (in.Mode() & os.ModeCharDevice) != 0, nil
}

func promptSelect(_ *bufio.Reader, label string, options []string, size int, startInSearchMode bool) (int, error) {
	sel := promptui.Select{
		Label:             label,
		Items:             options,
		Size:              minInt(len(options), size),
		StartInSearchMode: startInSearchMode,
	}
	i, _, err := sel.Run()
	if err != nil {
		return 0, err
	}
	return i, nil
}

func promptOptional(reader *bufio.Reader, label, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", label, defaultValue)
	} else {
		fmt.Printf("%s: ", label)
	}
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(text)
	if text == "" {
		return defaultValue
	}
	return text
}

func promptRequired(reader *bufio.Reader, label, defaultValue string) string {
	for {
		v := promptOptional(reader, label, defaultValue)
		if strings.TrimSpace(v) != "" {
			return v
		}
		fmt.Println("Value is required.")
	}
}

func promptConfirm(label string, defaultNo bool) (bool, error) {
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

// promptMultiSelectByDone is a dependency-free "multi-select" UX:
// users repeatedly pick one item from the list until they choose [Done].
func promptMultiSelectByDone(label string, items []string, size int) ([]string, error) {
	if len(items) == 0 {
		return nil, nil
	}

	remaining := append([]string{}, items...)
	sort.Strings(remaining)

	var selected []string
	for len(remaining) > 0 {
		menu := append([]string{"[Done]"}, remaining...)
		i, err := promptSelect(nil, label, menu, size, true)
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
