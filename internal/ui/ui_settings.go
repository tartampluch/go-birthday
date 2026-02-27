package ui

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/tartampluch/go-birthday/internal/config"
	"github.com/zalando/go-keyring"
)

// settingsWidgets holds references to UI elements to simplify data retrieval during save.
type settingsWidgets struct {
	langSelect    *widget.Select
	modeSelect    *widget.Select
	urlEntry      *widget.Entry
	userEntry     *widget.Entry
	passEntry     *widget.Entry
	pathEntry     *widget.Entry
	entryInterval *NumericalEntry
	entryPort     *NumericalEntry
	checkReminder *widget.Check
	entryRemValue *NumericalEntry
	selectRemUnit *widget.Select
	selectRemDir  *widget.Select
}

// ShowSettingsWindow displays the configuration dialog allowing users to manage settings.
func (app *GoBirthdayApp) ShowSettingsWindow() {
	if app.Window != nil {
		slog.Debug("Settings window already open, requesting focus", config.LogKeyComponent, config.CompUISet)
		app.Window.RequestFocus()
		return
	}

	slog.Info("Opening settings window", config.LogKeyComponent, config.CompUISet)
	w := app.App.NewWindow(app.GetMsg(config.TKeyWinTitle))
	app.Window = w

	// Initialize widgets container
	sw := &settingsWidgets{}

	// refreshLayout triggers a window resize based on content visibility.
	var refreshLayout func()
	onLayoutChange := func() {
		if refreshLayout != nil {
			refreshLayout()
		}
	}

	// Forward declaration for the save button.
	var btnSave *widget.Button

	// --- 1. Language ---
	sw.langSelect = widget.NewSelect(app.SupportedLanguages, nil)
	sw.langSelect.SetSelected(app.Preferences.StringWithFallback(config.PrefLanguage, config.DefaultLanguage))

	// --- 2. Source Section ---
	// Map translated strings to values is handled later.
	sw.modeSelect = widget.NewSelect([]string{
		app.GetMsg(config.TKeyModeCardDAV),
		app.GetMsg(config.TKeyModeLocal),
	}, nil)

	sw.urlEntry = widget.NewEntry()
	sw.urlEntry.SetText(app.Preferences.String(config.PrefCardDAVURL))
	sw.urlEntry.PlaceHolder = config.PlaceholderURL

	sw.userEntry = widget.NewEntry()
	sw.userEntry.SetText(app.Preferences.String(config.PrefUsername))

	sw.passEntry = widget.NewPasswordEntry()
	// Attempt to pre-fill password from secure storage
	if user := sw.userEntry.Text; user != "" {
		if pwd, err := keyring.Get(config.KeyringService, user); err == nil {
			sw.passEntry.SetText(pwd)
		}
	}

	sw.pathEntry = widget.NewEntry()
	sw.pathEntry.SetText(app.Preferences.String(config.PrefLocalPath))

	sourceCard := app.buildSourceCard(w, sw, onLayoutChange)

	// --- 3. General Section (Interval & Port) ---

	// Interval: Numerical only. No specific validator needed as "0" or "empty" are handled in save logic.
	sw.entryInterval = NewNumericalEntry()
	sw.entryInterval.SetText(strconv.Itoa(app.Preferences.IntWithFallback(config.PrefInterval, config.DefaultRefreshMin)))

	// Port: Numerical only, but requires strict Validation (Range 1-65535).
	sw.entryPort = NewNumericalEntry()
	sw.entryPort.SetText(app.Preferences.StringWithFallback(config.PrefServerPort, config.DefaultPort))
	sw.entryPort.Validator = func(s string) error {
		if s == "" {
			return errors.New(app.GetMsg(config.TKeyErrPortReq))
		}
		port, err := strconv.Atoi(s)
		if err != nil {
			return errors.New(app.GetMsg(config.TKeyErrPortNum))
		}
		if port < config.MinPort || port > config.MaxPort {
			return errors.New(app.GetMsg(config.TKeyErrPortRange))
		}
		return nil
	}

	// Construct the General Form
	itemLang := widget.NewFormItem(app.GetMsg(config.TKeyLblLanguage), sw.langSelect)
	itemLang.HintText = app.GetMsg(config.TKeyHelpLanguage)

	widInterval := container.NewBorder(nil, nil, nil, widget.NewLabel(app.GetMsg(config.TKeyLblMinutes)), sw.entryInterval)
	itemInterval := widget.NewFormItem(app.GetMsg(config.TKeyLblRefresh), widInterval)
	itemInterval.HintText = app.GetMsg(config.TKeyHelpInterval)

	itemPort := widget.NewFormItem(app.GetMsg(config.TKeyLblPort), sw.entryPort)
	itemPort.HintText = app.GetMsg(config.TKeyHelpPort)

	generalForm := widget.NewForm(itemLang, itemInterval, itemPort)
	generalCard := widget.NewCard(app.GetMsg(config.TKeyLblGeneral), "", generalForm)

	// --- 4. Reminder Section ---
	sw.checkReminder = widget.NewCheck(app.GetMsg(config.TKeyLblEnableRem), nil)
	sw.checkReminder.Checked = app.Preferences.Bool(config.PrefReminderEnabled)

	// Reminder Value: Numerical only. No validator needed; empty disables the feature in save logic.
	sw.entryRemValue = NewNumericalEntry()
	// Fallback uses constant DefaultReminderValue instead of literal "1"
	sw.entryRemValue.SetText(strconv.Itoa(app.Preferences.IntWithFallback(config.PrefReminderValue, config.DefaultReminderValue)))

	sw.selectRemUnit = widget.NewSelect([]string{
		app.GetMsg(config.TKeyUnitDays),
		app.GetMsg(config.TKeyUnitHours),
		app.GetMsg(config.TKeyUnitMinutes),
	}, nil)

	// Determine initial selection for Unit based on preferences
	currentUnit := app.Preferences.StringWithFallback(config.PrefReminderUnit, config.UnitDays)
	switch currentUnit {
	case config.UnitHours:
		sw.selectRemUnit.SetSelected(app.GetMsg(config.TKeyUnitHours))
	case config.UnitMinutes:
		sw.selectRemUnit.SetSelected(app.GetMsg(config.TKeyUnitMinutes))
	default:
		sw.selectRemUnit.SetSelected(app.GetMsg(config.TKeyUnitDays))
	}

	sw.selectRemDir = widget.NewSelect([]string{
		app.GetMsg(config.TKeyDirBefore),
		app.GetMsg(config.TKeyDirAfter),
	}, nil)
	// Determine initial selection for Direction
	currentDir := app.Preferences.StringWithFallback(config.PrefReminderDir, config.DirBefore)
	if currentDir == config.DirAfter {
		sw.selectRemDir.SetSelected(app.GetMsg(config.TKeyDirAfter))
	} else {
		sw.selectRemDir.SetSelected(app.GetMsg(config.TKeyDirBefore))
	}

	notifCard := app.buildNotifCard(sw, onLayoutChange)

	// --- Actions ---
	saveAction := func() {
		// Only the Port field has a strict requirement that blocks saving if invalid.
		if err := sw.entryPort.Validate(); err != nil {
			dialog.ShowError(err, w)
			return
		}
		app.saveSettings(sw, w)
	}

	btnSave = widget.NewButtonWithIcon(app.GetMsg(config.TKeyBtnSave), theme.DocumentSaveIcon(), saveAction)
	btnSave.Importance = widget.HighImportance
	btnCancel := widget.NewButtonWithIcon(app.GetMsg(config.TKeyBtnCancel), theme.CancelIcon(), func() { w.Close() })

	// --- Footer ---
	footerText := fmt.Sprintf(app.GetMsg(config.TKeyLblFooter), config.Version)
	footerLabel := widget.NewLabel(footerText)
	footerLabel.Alignment = fyne.TextAlignCenter
	footerLabel.TextStyle = fyne.TextStyle{Italic: true}

	// Assembly
	paddedContent := container.NewPadded(container.NewVBox(
		sourceCard,
		generalCard,
		notifCard,
		// Using constant for columns
		container.NewGridWithColumns(config.LayoutColumnsDouble, btnCancel, btnSave),
		footerLabel,
	))

	// Logic to resize window based on content
	refreshLayout = func() {
		paddedContent.Refresh()
		minSize := paddedContent.MinSize()
		w.Resize(fyne.NewSize(config.SettingsWindowWidth, minSize.Height))
	}

	w.SetContent(paddedContent)
	w.SetFixedSize(true)
	w.SetOnClosed(func() { app.Window = nil })

	// Initial layout calculation
	refreshLayout()
	w.Show()
}

// buildSourceCard constructs the source selection UI.
func (app *GoBirthdayApp) buildSourceCard(w fyne.Window, sw *settingsWidgets, onLayoutChange func()) *widget.Card {
	browseBtn := widget.NewButton(app.GetMsg(config.TKeyBtnBrowse), func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err == nil && r != nil {
				sw.pathEntry.SetText(r.URI().Path())
			}
		}, w)
		// Use file extension constants from config
		d.SetFilter(storage.NewExtensionFileFilter([]string{config.ExtVCF, config.ExtVCard}))
		d.Show()
	})

	// Web Form
	itemURL := widget.NewFormItem(app.GetMsg(config.TKeyLblURL), sw.urlEntry)
	itemURL.HintText = app.GetMsg(config.TKeyHelpURL)

	itemUser := widget.NewFormItem(app.GetMsg(config.TKeyLblUser), sw.userEntry)
	itemPass := widget.NewFormItem(app.GetMsg(config.TKeyLblPass), sw.passEntry)

	webForm := widget.NewForm(itemURL, itemUser, itemPass)

	// Local Form
	localForm := container.NewBorder(nil, nil, nil, browseBtn, sw.pathEntry)

	// Dynamic visibility based on mode
	updateVis := func(mode string) {
		if mode == app.GetMsg(config.TKeyModeLocal) {
			webForm.Hide()
			localForm.Show()
		} else {
			webForm.Show()
			localForm.Hide()
		}
		if onLayoutChange != nil {
			onLayoutChange()
		}
	}
	sw.modeSelect.OnChanged = updateVis

	// Set initial state
	currentMode := app.Preferences.String(config.PrefSourceMode)
	if currentMode == config.SourceModeLocal {
		sw.modeSelect.SetSelected(app.GetMsg(config.TKeyModeLocal))
	} else {
		sw.modeSelect.SetSelected(app.GetMsg(config.TKeyModeCardDAV))
	}

	// Apply initial visibility
	if sw.modeSelect.Selected == app.GetMsg(config.TKeyModeLocal) {
		webForm.Hide()
		localForm.Show()
	} else {
		webForm.Show()
		localForm.Hide()
	}

	return widget.NewCard(app.GetMsg(config.TKeyLblSource), "", container.NewVBox(sw.modeSelect, webForm, localForm))
}

// buildNotifCard constructs the notification/reminder UI.
func (app *GoBirthdayApp) buildNotifCard(sw *settingsWidgets, onLayoutChange func()) *widget.Card {
	lblStart := widget.NewLabel(app.GetMsg(config.TKeyLblStartDay))

	// Controls: Value | Unit | Direction | "Start of day"
	controls := container.NewHBox(sw.selectRemUnit, sw.selectRemDir, lblStart)
	row := container.NewBorder(nil, nil, nil, controls, sw.entryRemValue)

	sw.checkReminder.OnChanged = func(b bool) {
		if b {
			row.Show()
		} else {
			row.Hide()
		}
		if onLayoutChange != nil {
			onLayoutChange()
		}
	}

	if sw.checkReminder.Checked {
		row.Show()
	} else {
		row.Hide()
	}

	return widget.NewCard(app.GetMsg(config.TKeyLblNotif), "", container.NewVBox(sw.checkReminder, row))
}

// saveSettings persists the data and triggers a sync.
// It handles logic for disabling features if numeric fields are empty.
func (app *GoBirthdayApp) saveSettings(sw *settingsWidgets, w fyne.Window) {
	slog.Info("Saving preferences", config.LogKeyComponent, config.CompUISet)

	// Helper to map UI strings back to config constants
	modeMap := map[string]string{
		app.GetMsg(config.TKeyModeCardDAV): config.SourceModeWeb,
		app.GetMsg(config.TKeyModeLocal):   config.SourceModeLocal,
	}

	app.Preferences.SetString(config.PrefLanguage, sw.langSelect.Selected)
	app.Preferences.SetString(config.PrefSourceMode, modeMap[sw.modeSelect.Selected])
	app.Preferences.SetString(config.PrefCardDAVURL, sw.urlEntry.Text)
	app.Preferences.SetString(config.PrefUsername, sw.userEntry.Text)
	app.Preferences.SetString(config.PrefLocalPath, sw.pathEntry.Text)

	// Save password to Keyring only if provided
	if sw.userEntry.Text != "" && sw.passEntry.Text != "" {
		if err := keyring.Set(config.KeyringService, sw.userEntry.Text, sw.passEntry.Text); err != nil {
			slog.Error("Failed to save credentials to keyring", config.LogKeyError, err, config.LogKeyComponent, config.CompUISet)
		}
	}

	// Logic: Interval
	// If empty or 0, we treat it as disabled (0).
	intervalText := sw.entryInterval.Text
	if intervalText == "" || intervalText == "0" {
		app.Preferences.SetInt(config.PrefInterval, config.DisabledInterval)
		slog.Info("Auto-refresh disabled via settings", config.LogKeyComponent, config.CompUISet)
	} else {
		if i, err := strconv.Atoi(intervalText); err == nil {
			app.Preferences.SetInt(config.PrefInterval, i)
		}
	}

	// Port
	if sw.entryPort.Text != "" {
		app.Preferences.SetString(config.PrefServerPort, sw.entryPort.Text)
	}

	// Logic: Reminder
	// If the value field is empty, we force disable reminders, even if the checkbox is checked.
	remValueText := sw.entryRemValue.Text
	if remValueText == "" {
		app.Preferences.SetBool(config.PrefReminderEnabled, false)
		slog.Info("Reminders disabled via settings (value is empty)", config.LogKeyComponent, config.CompUISet)
	} else {
		// Otherwise, respect the checkbox state
		app.Preferences.SetBool(config.PrefReminderEnabled, sw.checkReminder.Checked)
		if v, err := strconv.Atoi(remValueText); err == nil {
			app.Preferences.SetInt(config.PrefReminderValue, v)
		}
	}

	// Map Unit UI String -> Config Code (d, h, m)
	unit := config.UnitDays // default
	switch sw.selectRemUnit.Selected {
	case app.GetMsg(config.TKeyUnitHours):
		unit = config.UnitHours
	case app.GetMsg(config.TKeyUnitMinutes):
		unit = config.UnitMinutes
	}
	app.Preferences.SetString(config.PrefReminderUnit, unit)

	// Map Direction UI String -> Config Code (before, after)
	dir := config.DirBefore // default
	if sw.selectRemDir.Selected == app.GetMsg(config.TKeyDirAfter) {
		dir = config.DirAfter
	}
	app.Preferences.SetString(config.PrefReminderDir, dir)

	// Trigger system-wide updates
	app.UpdateLocalizer()
	app.RefreshTrayMenu()
	app.performSync(true) // Force immediate sync with new settings

	w.Close()
}
