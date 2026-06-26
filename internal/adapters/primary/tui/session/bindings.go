package session

import "charm.land/bubbles/v2/key"

const jumpRowDelta = 5

var (
	popupNextFieldKeyBinding       = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
	popupPrevFieldKeyBinding       = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous field"))
	popupSubmitFormKeyBinding      = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "create session"))
	popupCloseKeyBinding           = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel"))
	popupSelectorNextKeyBinding    = key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next option"))
	popupSelectorPrevKeyBinding    = key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "previous option"))
	popupToggleKeyBinding          = key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle"))
	repoPickerEnterPasteKeyBinding  = key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit / new path"))
	repoPickerExitPasteKeyBinding   = key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "back to list"))
	repoPickerEnterSearchKeyBinding = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search"))
	branchPickerUpKeyBinding       = key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "prev branch"))
	branchPickerDownKeyBinding     = key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "next branch"))
	jumpUpKeyBinding               = key.NewBinding(key.WithKeys("ctrl+up"), key.WithHelp("ctrl+↑", "jump up"))
	jumpDownKeyBinding             = key.NewBinding(key.WithKeys("ctrl+down"), key.WithHelp("ctrl+↓", "jump down"))
	ReorderSessionUpKeyBinding     = key.NewBinding(key.WithKeys("shift+up"), key.WithHelp("shift+↑", "move session up"))
	ReorderSessionDownKeyBinding   = key.NewBinding(key.WithKeys("shift+down"), key.WithHelp("shift+↓", "move session down"))
	GoToNextGroupKeyBinding        = key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "go to next group"))
	GoToPrevGroupKeyBinding        = key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "go to previous group"))
	CycleLabelKeyBinding           = key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "cycle labels"))
	RenameKeyBinding               = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename"))
	DeleteSessionKeyBinding        = key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete session"))
	deleteConfirmKeyBinding        = key.NewBinding(key.WithKeys("y", "enter"), key.WithHelp("y/enter", "confirm delete"))
	deleteCancelKeyBinding         = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "cancel"))
)
