package gsso

import (
	"os"

	"github.com/charmbracelet/huh"
	xterm "github.com/charmbracelet/x/term"
)

// RequireTTY returns an error naming what needs an interactive terminal, so
// scripted/CI callers get told to pass names as arguments instead of hanging
// on a menu.
func RequireTTY(what string) error {
	if !xterm.IsTerminal(os.Stdin.Fd()) {
		return NewErrorWithFix(ExTTY, "pass the names as arguments instead",
			"%s needs an interactive terminal", what)
	}
	return nil
}

func theme() *huh.Theme {
	return huh.ThemeCharm()
}

// ChooseMultiple runs a searchable checkbox menu and returns the selected
// options, in the order they were displayed. preselected marks options that
// start checked.
func ChooseMultiple(title string, options []string, preselected []string) ([]string, error) {
	if err := RequireTTY(title); err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, NewError("nothing to choose from")
	}
	pre := profileSet(preselected)
	huhOptions := make([]huh.Option[string], len(options))
	for i, o := range options {
		huhOptions[i] = huh.NewOption(o, o).Selected(pre[o])
	}
	var selected []string
	field := huh.NewMultiSelect[string]().
		Title(title).
		Description("space: toggle   /: filter   enter: confirm   esc: cancel").
		Options(huhOptions...).
		Filterable(true).
		Height(minInt(len(options)+4, 18)).
		Value(&selected)
	if err := field.WithTheme(theme()).Run(); err != nil {
		return nil, mapHuhErr(err)
	}
	if len(selected) == 0 {
		return nil, NewError("nothing selected")
	}
	return selected, nil
}

// ChooseOne runs a searchable single-select menu.
func ChooseOne(title string, options []string) (string, error) {
	if err := RequireTTY(title); err != nil {
		return "", err
	}
	if len(options) == 0 {
		return "", NewError("nothing to choose from")
	}
	huhOptions := make([]huh.Option[string], len(options))
	for i, o := range options {
		huhOptions[i] = huh.NewOption(o, o)
	}
	var chosen string
	field := huh.NewSelect[string]().
		Title(title).
		Description("/: filter   enter: select   esc: cancel").
		Options(huhOptions...).
		Height(minInt(len(options)+4, 18)).
		Value(&chosen)
	if err := field.WithTheme(theme()).Run(); err != nil {
		return "", mapHuhErr(err)
	}
	return chosen, nil
}

// ChooseMultipleWithDefault runs the checkbox menu, then (if anything was
// selected) a second single-select to mark which of the selected items is
// the default. currentDefault, if non-empty and still selected, is
// pre-highlighted.
func ChooseMultipleWithDefault(title string, options []string, preselected []string, currentDefault string) (selected []string, def string, err error) {
	selected, err = ChooseMultiple(title, options, preselected)
	if err != nil {
		return nil, "", err
	}
	if len(selected) == 1 {
		return selected, selected[0], nil
	}
	def, err = ChooseOne("Default among the selected", selected)
	if err != nil {
		return nil, "", err
	}
	return selected, def, nil
}

// Confirm asks a yes/no question.
func Confirm(question string) (bool, error) {
	if err := RequireTTY("this confirmation"); err != nil {
		return false, err
	}
	var yes bool
	field := huh.NewConfirm().
		Title(question).
		Affirmative("Yes").
		Negative("No").
		Value(&yes)
	if err := field.WithTheme(theme()).Run(); err != nil {
		return false, mapHuhErr(err)
	}
	return yes, nil
}

func mapHuhErr(err error) error {
	if err == huh.ErrUserAborted {
		return NewError("selection cancelled")
	}
	return err
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
