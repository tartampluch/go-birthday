package ui

import (
	"embed"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/tartampluch/go-birthday/internal/config"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

// SetupI18n initializes the translation bundle and detects available languages.
func (app *GoBirthdayApp) SetupI18n() {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		slog.Error(config.ErrLocalesAccess,
			config.LogKeyComponent, config.CompI18n,
			config.LogKeyError, err,
		)
		return
	}

	var detectedLangs []string

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "active.") || !strings.HasSuffix(name, ".json") {
			slog.Debug(config.MsgLocaleSkip,
				config.LogKeyComponent, config.CompI18n,
				config.LogKeyFile, name,
			)
			continue
		}

		trimmed := strings.TrimPrefix(name, "active.")
		langCode := strings.TrimSuffix(trimmed, ".json")

		if langCode == "" {
			slog.Warn(config.MsgLocaleBadName,
				config.LogKeyComponent, config.CompI18n,
				config.LogKeyFile, name,
			)
			continue
		}

		detectedLangs = append(detectedLangs, langCode)

		path := "locales/" + name
		if _, err := bundle.LoadMessageFileFS(localeFS, path); err != nil {
			slog.Error(config.ErrLocaleLoad,
				config.LogKeyComponent, config.CompI18n,
				config.LogKeyFile, name,
				config.LogKeyError, err,
			)
		} else {
			slog.Debug(config.MsgLocaleLoaded,
				config.LogKeyComponent, config.CompI18n,
				config.LogKeyLang, langCode,
				config.LogKeyFile, name,
			)
		}
	}

	app.SupportedLanguages = detectedLangs
	app.I18nBundle = bundle
	app.UpdateLocalizer()
}

// UpdateLocalizer refreshes the translator based on the user's language preference.
func (app *GoBirthdayApp) UpdateLocalizer() {
	lang := app.Preferences.String(config.PrefLanguage)
	if lang == "" {
		lang = config.DefaultLanguage
	}
	app.Localizer = i18n.NewLocalizer(app.I18nBundle, lang)
}

// GetMsg is a helper to translate a key safely.
func (app *GoBirthdayApp) GetMsg(key string) string {
	if app.Localizer == nil {
		return key
	}
	msg, err := app.Localizer.Localize(&i18n.LocalizeConfig{MessageID: key})
	if err != nil {
		slog.Debug(config.MsgTransMissing,
			config.LogKeyComponent, config.CompI18n,
			config.LogKeyKey, key,
			config.LogKeyError, err,
		)
		return key
	}
	return msg
}
