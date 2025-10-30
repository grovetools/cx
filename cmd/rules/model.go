package rules

import "github.com/mattsolo1/grove-core/tui/components/help"

type ruleItem struct {
	name   string
	path   string
	active bool
}

type rulesPickerModel struct {
	items         []ruleItem
	selectedIndex int
	keys          pickerKeyMap
	help          help.Model
	width, height int
	err           error
	quitting      bool
}

func newRulesPickerModel() *rulesPickerModel {
	return &rulesPickerModel{
		keys: defaultPickerKeyMap,
		help: help.New(defaultPickerKeyMap),
	}
}
