package ui

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/tartampluch/go-birthday/internal/config"
	"github.com/tartampluch/go-birthday/internal/engine"
)

// ShowContactsWindow displays a window with all birthdays sorted by next occurrence.
// It implements a singleton pattern: if the window is already open, it requests focus.
// It uses native Fyne table headers for sorting interaction.
func (app *GoBirthdayApp) ShowContactsWindow() {
	if app.contactsWindow != nil {
		app.contactsWindow.RequestFocus()
		return
	}

	title := app.GetMsg(config.TKeyWinContacts)
	app.contactsWindow = app.App.NewWindow(title)
	app.contactsWindow.Resize(fyne.NewSize(config.ContactsWinWidth, config.ContactsWinHeight))

	// Create a local copy of contacts for sorting/display to avoid race conditions
	app.ContactsMut.RLock()
	displayContacts := make([]engine.BirthdayEntry, len(app.Contacts))
	copy(displayContacts, app.Contacts)
	app.ContactsMut.RUnlock()

	slog.Info(config.LogMsgOpenWin,
		config.LogKeyComponent, config.CompUI,
		config.LogKeyCount, len(displayContacts))

	// Internal Sorting State
	currentSortCol := config.ColIDDate
	sortAsc := true

	var refreshTable func()

	// performSort applies the sorting logic based on the selected column.
	performSort := func() {
		sort.Slice(displayContacts, func(i, j int) bool {
			a, b := displayContacts[i], displayContacts[j]
			var less bool
			switch currentSortCol {
			case config.ColIDName:
				less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
			case config.ColIDAge:
				// Handle contacts with unknown birth years (YearKnown = false)
				if !a.YearKnown && b.YearKnown {
					less = false // "Unknown" > "Known" (Push to bottom in ASC)
				} else if a.YearKnown && !b.YearKnown {
					less = true
				} else {
					less = a.AgeNext < b.AgeNext
				}
			default: // config.ColIDDate
				if a.NextOccurrence.Equal(b.NextOccurrence) {
					// Secondary sort key: Name
					less = a.Name < b.Name
				} else {
					less = a.NextOccurrence.Before(b.NextOccurrence)
				}
			}

			if !sortAsc {
				return !less
			}
			return less
		})

		slog.Debug(config.LogMsgSorted,
			config.LogKeyComponent, config.CompUI,
			config.LogKeySortCol, currentSortCol,
			config.LogKeySortAsc, sortAsc)
	}

	// Initial sort (Default: By Date, Ascending)
	performSort()

	// --- UI Table Component ---

	table := widget.NewTable(
		// Length callback
		func() (int, int) {
			return len(displayContacts), 3
		},
		// Create cell callback
		func() fyne.CanvasObject {
			return widget.NewLabel(config.TablePlaceholder)
		},
		// Update cell callback
		func(id widget.TableCellID, o fyne.CanvasObject) {
			label := o.(*widget.Label)
			if id.Row >= len(displayContacts) {
				return
			}
			c := displayContacts[id.Row]

			switch id.Col {
			case config.ColIDName:
				label.SetText(c.Name)
			case config.ColIDDate:
				// Retrieve the localized date format
				format := app.GetMsg(config.TKeyFormatDate)
				if format == config.TKeyFormatDate {
					format = config.DateFormatDisplay
				}
				label.SetText(c.NextOccurrence.Format(format))

			case config.ColIDAge:
				if c.YearKnown {
					if c.AgeNext == 0 {
						// Born this year (very rare case for upcoming list unless date is exact match today for a newborn)
						label.SetText(config.AgeBirth)
					} else {
						// Show transition: "PrevAge -> NextAge"
						prevAge := c.AgeNext - 1
						if prevAge == 0 {
							// Special case: "Birth -> 1"
							birthText := app.GetMsg(config.TKeyAgeBirth)
							if birthText == config.TKeyAgeBirth {
								birthText = "Birth" // Fallback
							}
							label.SetText(fmt.Sprintf("%s → %d", birthText, c.AgeNext))
						} else {
							// Standard case: "25 -> 26"
							label.SetText(fmt.Sprintf("%d → %d", prevAge, c.AgeNext))
						}
					}
				} else {
					label.SetText(config.AgeUnknown)
				}
			}
		},
	)

	// --- Header Configuration (Fyne Native) ---

	table.ShowHeaderRow = true

	// CreateHeader returns a button for interactivity.
	table.CreateHeader = func() fyne.CanvasObject {
		return widget.NewButton("Header", func() {})
	}

	// UpdateHeader sets the localized title and visual sort indicator.
	table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		btn := o.(*widget.Button)

		var titleKey string
		switch id.Col {
		case config.ColIDName:
			titleKey = config.TKeyColName
		case config.ColIDDate:
			titleKey = config.TKeyColDate
		case config.ColIDAge:
			titleKey = config.TKeyColAge
		}

		text := app.GetMsg(titleKey)

		// Append sort indicator if this is the active column
		if id.Col == currentSortCol {
			if sortAsc {
				text += config.SortIconAsc
			} else {
				text += config.SortIconDesc
			}
		}

		btn.SetText(text)

		// Set OnTapped handler to trigger sorting
		btn.OnTapped = func() {
			if currentSortCol == id.Col {
				sortAsc = !sortAsc
			} else {
				currentSortCol = id.Col
				sortAsc = true
			}
			refreshTable()
		}
	}

	// Set column widths based on configuration
	table.SetColumnWidth(config.ColIDName, config.ColWidthName)
	table.SetColumnWidth(config.ColIDDate, config.ColWidthDate)
	table.SetColumnWidth(config.ColIDAge, config.ColWidthAge)

	refreshTable = func() {
		performSort()
		table.Refresh()
	}

	// Layout Assembly
	content := container.NewBorder(nil, nil, nil, nil, table)
	app.contactsWindow.SetContent(content)

	// Cleanup on close
	app.contactsWindow.SetOnClosed(func() {
		app.contactsWindow = nil
	})

	app.contactsWindow.Show()
}
