package repo

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/kohbis/xr/internal/interactive"
)

func promptSelect(_ *bufio.Reader, label string, options []string, size int, startInSearchMode bool) (int, error) {
	return interactive.Select(label, options, size, startInSearchMode)
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
	return interactive.Confirm(label, defaultNo)
}

func promptYesNoSelect(label string, defaultNo bool) (bool, error) {
	return interactive.YesNo(label, defaultNo)
}

func promptMultiSelectByDone(label string, items []string, size int) ([]string, error) {
	return interactive.MultiSelectByDone(label, items, size)
}
