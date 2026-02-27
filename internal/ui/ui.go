package ui

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/tartampluch/go-birthday/internal/config"
	"github.com/tartampluch/go-birthday/internal/engine"
	"github.com/tartampluch/go-birthday/internal/server"
	"github.com/zalando/go-keyring"
)

//go:embed Icon.png
var appIconData []byte

// GoBirthdayApp encapsulates the UI state, preferences, and background logic.
type GoBirthdayApp struct {
	App         fyne.App
	Window      fyne.Window
	Preferences fyne.Preferences
	I18nBundle  *i18n.Bundle
	Localizer   *i18n.Localizer
	Ctx         context.Context

	Server  *server.CalendarServer
	Fetcher engine.VCardFetcher
	Clock   engine.Clock // Injected clock for testability (e.g. mocking time travel)

	Tray desktop.App
	Menu *fyne.Menu

	TrayStatusItem   *fyne.MenuItem
	TrayRefreshItem  *fyne.MenuItem
	TraySettingsItem *fyne.MenuItem

	SupportedLanguages []string
	configChan         chan string

	// Contacts State
	ContactsMut    sync.RWMutex
	Contacts       []engine.BirthdayEntry
	contactsWindow fyne.Window
}

// NewGoBirthdayApp constructs the application and wires dependencies.
func NewGoBirthdayApp(a fyne.App, ctx context.Context, srv *server.CalendarServer, fetcher engine.VCardFetcher) *GoBirthdayApp {
	a.SetIcon(fyne.NewStaticResource(config.IconFile, appIconData))

	return &GoBirthdayApp{
		App:                a,
		Preferences:        a.Preferences(),
		Ctx:                ctx,
		Server:             srv,
		Fetcher:            fetcher,
		Clock:              engine.RealClock{}, // Default to real clock in production
		SupportedLanguages: config.SupportedLanguages,
		configChan:         make(chan string, config.ChannelBufferSize),
		Contacts:           make([]engine.BirthdayEntry, 0),
	}
}

// Run launches the application services and the main UI loop.
func (app *GoBirthdayApp) Run() {
	app.SetupI18n()
	app.watchPreferences()

	go func() {
		slog.Info(config.MsgServerListen,
			config.LogKeyPort, app.Server.Port,
			config.LogKeyComponent, config.CompUI)

		if err := app.Server.Start(app.Ctx); err != nil {
			slog.Error(config.ErrServerStartup,
				config.LogKeyError, err,
				config.LogKeyComponent, config.CompUI)

			app.App.SendNotification(fyne.NewNotification(
				config.TitleStartupError,
				fmt.Sprintf(config.MsgPortBusy, app.Server.Port)))
		}
	}()

	if desk, ok := app.App.(desktop.App); ok {
		app.Tray = desk
		app.Tray.SetSystemTrayIcon(app.App.Icon())
		app.setupTrayMenu()
	} else {
		slog.Warn(config.ErrTrayNotSupported,
			config.LogKeyComponent, config.CompUI)
	}

	go app.backgroundWorker()
	app.App.Run()
}

// watchPreferences monitors changes to settings to trigger immediate updates.
func (app *GoBirthdayApp) watchPreferences() {
	app.Preferences.AddChangeListener(func() {
		select {
		case app.configChan <- config.PrefInterval:
		default:
		}
	})
}

// setupTrayMenu constructs the system tray menu.
func (app *GoBirthdayApp) setupTrayMenu() {
	// Status Item now acts as a button to open Contacts Window
	app.TrayStatusItem = fyne.NewMenuItem(config.FallbackTrayLabel, func() {
		app.ShowContactsWindow()
	})
	// Removed Disabled=true so user can click it
	app.TrayStatusItem.Disabled = false

	app.TrayRefreshItem = fyne.NewMenuItem(app.GetMsg(config.TKeyMenuRefresh), func() {
		go app.performSync(true)
	})

	app.TraySettingsItem = fyne.NewMenuItem(app.GetMsg(config.TKeyMenuSettings), func() {
		app.ShowSettingsWindow()
	})

	app.Menu = fyne.NewMenu(config.AppName,
		app.TrayStatusItem,
		fyne.NewMenuItemSeparator(),
		app.TrayRefreshItem,
		app.TraySettingsItem,
	)

	if app.Tray != nil {
		app.Tray.SetSystemTrayMenu(app.Menu)
	}
}

// RefreshTrayMenu updates localized labels in the tray menu.
func (app *GoBirthdayApp) RefreshTrayMenu() {
	if app.Menu == nil {
		return
	}
	app.TrayRefreshItem.Label = app.GetMsg(config.TKeyMenuRefresh)
	app.TraySettingsItem.Label = app.GetMsg(config.TKeyMenuSettings)
	app.Menu.Refresh()
}

// backgroundWorker manages the periodic synchronization schedule.
func (app *GoBirthdayApp) backgroundWorker() {
	log := slog.With(config.LogKeyComponent, config.CompWorker)

	app.performSync(false)

	getInterval := func() time.Duration {
		val := app.Preferences.IntWithFallback(config.PrefInterval, config.DefaultRefreshMin)
		if val <= 0 {
			val = config.DefaultRefreshMin
		}
		return time.Duration(val) * time.Minute
	}

	currentDuration := getInterval()
	ticker := time.NewTicker(currentDuration)
	defer ticker.Stop()

	log.Info(config.MsgWorkerStart, config.LogKeyInterval, currentDuration)

	for {
		select {
		case <-app.Ctx.Done():
			log.Info(config.MsgWorkerStop)
			return

		case <-app.configChan:
			newDuration := getInterval()
			if newDuration != currentDuration {
				log.Info(config.MsgUpdateSync, config.LogKeyOld, currentDuration, config.LogKeyNew, newDuration)
				currentDuration = newDuration
				ticker.Reset(currentDuration)
			}

		case <-ticker.C:
			app.performSync(false)
		}
	}
}

// performSync executes the business logic pipeline (Fetch -> Parse -> Generate).
func (app *GoBirthdayApp) performSync(manual bool) {
	slog.Info(config.MsgSyncReq,
		config.LogKeyComponent, config.CompUI,
		config.LogKeyManual, manual)

	if manual {
		app.App.SendNotification(fyne.NewNotification(config.AppName, app.GetMsg(config.TKeyNotifStart)))
	}

	cfg := app.loadSyncConfig()

	// Use the app's injected clock (Real or Mock)
	gen := &engine.Generator{
		Clock:         app.Clock,
		Fetcher:       app.Fetcher,
		FormatSummary: app.buildSummaryFormatter(),
	}

	icsData, contacts, countToday, err := gen.RunSync(app.Ctx, cfg)
	if err != nil {
		slog.Error(config.MsgSyncFailed, config.LogKeyError, err, config.LogKeyComponent, config.CompUI)
		if manual {
			app.App.SendNotification(fyne.NewNotification(config.TitleSyncError, app.GetMsg(config.TKeyNotifError)))
		}
		app.updateTrayStatus(-1)
		return
	}

	// Thread-safe update of contacts
	app.ContactsMut.Lock()
	app.Contacts = contacts
	app.ContactsMut.Unlock()

	app.Server.Update(icsData)
	app.updateTrayStatus(countToday)

	if manual {
		app.App.SendNotification(fyne.NewNotification(config.AppName, app.GetMsg(config.TKeyNotifSuccess)))
	}
}

// updateTrayStatus updates the top menu item to show how many birthdays are today.
func (app *GoBirthdayApp) updateTrayStatus(count int) {
	if app.Menu == nil || app.TrayStatusItem == nil {
		return
	}

	var label string
	if count < 0 {
		label = config.FallbackTrayError
	} else if count == 0 {
		// Explicit handling for 0 to use "No birthdays" / "Aucun anniversaire"
		label = app.GetMsg(config.TKeyTrayStatusZero)
		if label == config.TKeyTrayStatusZero {
			// Fallback logic if key is missing (though it shouldn't be)
			label = fmt.Sprintf(config.FallbackTrayDefault, 0)
		}
	} else {
		// Standard pluralization for > 0
		if app.Localizer != nil {
			msg, err := app.Localizer.Localize(&i18n.LocalizeConfig{
				MessageID:    config.TKeyTrayStatus,
				TemplateData: map[string]interface{}{"Count": count},
				PluralCount:  count,
			})
			if err == nil {
				label = msg
			}
		}
		if label == "" {
			label = fmt.Sprintf(config.FallbackTrayDefault, count)
		}
	}

	app.TrayStatusItem.Label = label
	app.Menu.Refresh()
}

// loadSyncConfig assembles the engine configuration from UI preferences and Keyring.
func (app *GoBirthdayApp) loadSyncConfig() engine.SyncConfig {
	cfg := engine.SyncConfig{
		Mode:      app.Preferences.String(config.PrefSourceMode),
		LocalPath: app.Preferences.String(config.PrefLocalPath),
		WebURL:    app.Preferences.String(config.PrefCardDAVURL),
		WebUser:   app.Preferences.String(config.PrefUsername),
	}

	if cfg.WebUser != "" {
		if p, err := keyring.Get(config.KeyringService, cfg.WebUser); err == nil {
			cfg.WebPass = p
		} else {
			slog.Debug(config.MsgPassFail,
				config.LogKeyUser, cfg.WebUser,
				config.LogKeyError, err,
				config.LogKeyComponent, config.CompUI)
		}
	}

	if app.Preferences.Bool(config.PrefReminderEnabled) {
		val := app.Preferences.IntWithFallback(config.PrefReminderValue, config.DefaultReminderValue)
		unit := app.Preferences.StringWithFallback(config.PrefReminderUnit, config.UnitDays)
		dir := app.Preferences.StringWithFallback(config.PrefReminderDir, config.DirBefore)

		sign := config.ISOPeriodPrefix
		if dir == config.DirBefore {
			sign = config.ISONegativePrefix
		}

		switch unit {
		case config.UnitHours:
			cfg.ReminderTrigger = fmt.Sprintf("%s%d%s", sign, val, config.ISOHour)
		case config.UnitMinutes:
			cfg.ReminderTrigger = fmt.Sprintf("%s%d%s", sign, val, config.ISOMinute)
		default:
			cfg.ReminderTrigger = fmt.Sprintf("%s%d%s", sign, val, config.ISODay)
		}
	}

	return cfg
}

// buildSummaryFormatter returns a closure that localizes the event summary.
func (app *GoBirthdayApp) buildSummaryFormatter() func(name string, age int, yearKnown bool) string {
	return func(name string, age int, yearKnown bool) string {
		var msg string
		var err error

		if app.Localizer != nil {
			if yearKnown {
				// Special Case: Age 0 means "Birth"
				if age == 0 {
					msg, err = app.Localizer.Localize(&i18n.LocalizeConfig{
						MessageID:    config.TKeyEvtSummaryBirth,
						TemplateData: map[string]interface{}{"Name": name},
					})
				} else {
					msg, err = app.Localizer.Localize(&i18n.LocalizeConfig{
						MessageID:    config.TKeyEvtSummaryAge,
						TemplateData: map[string]interface{}{"Name": name, "Age": age},
					})
				}
			} else {
				msg, err = app.Localizer.Localize(&i18n.LocalizeConfig{
					MessageID:    config.TKeyEvtSummary,
					TemplateData: map[string]interface{}{"Name": name},
				})
			}
		} else {
			// Using the constant error message for consistency
			err = fmt.Errorf(config.ErrLocNotInit)
		}

		if err != nil || msg == "" {
			if yearKnown {
				if age == 0 {
					return fmt.Sprintf(config.FallbackSummaryBirth, name)
				}
				return fmt.Sprintf(config.FallbackSummaryAge, name, age)
			}
			return fmt.Sprintf(config.FallbackSummary, name)
		}
		return msg
	}
}
