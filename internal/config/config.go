package config

import (
	"io/fs"
	"time"
)

// -----------------------------------------------------------------------------
// Build Information
// -----------------------------------------------------------------------------

// Build variables are injected via -ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// UserAgent identifies the HTTP client.
var UserAgent = "Go-Birthday/" + Version

// -----------------------------------------------------------------------------
// Application Constants
// -----------------------------------------------------------------------------

const (
	AppName           = "Go Birthday"
	AppID             = "com.github.tartampluch.go-birthday"
	KeyringService    = "com.github.tartampluch.go-birthday"
	LocalhostBindAddr = "127.0.0.1"
	LogFileName       = "app.log"
	IconFile          = "Icon.png"
)

// -----------------------------------------------------------------------------
// Exit Codes
// -----------------------------------------------------------------------------

const (
	ExitCodeSuccess = 0
	ExitCodeError   = 1
)

// -----------------------------------------------------------------------------
// System & File Permissions
// -----------------------------------------------------------------------------

const (
	// FilePermUserRW represents -rw------- (Read/Write for owner only).
	// Used for sensitive files like logs.
	FilePermUserRW fs.FileMode = 0600

	// DirPermUserRWX represents drwx------ (Read/Write/Exec for owner only).
	// Used for creating secure cache directories.
	DirPermUserRWX fs.FileMode = 0700

	// ChannelBufferSize defines the standard buffer size for internal signaling channels.
	ChannelBufferSize = 1
)

// -----------------------------------------------------------------------------
// CLI Flags & Descriptions
// -----------------------------------------------------------------------------

const (
	FlagVersion      = "version"
	FlagDebug        = "debug"
	FlagDescVersion  = "Show application version and exit"
	FlagDescDebug    = "Enable debug logging to stdout"
	MsgVersionOutput = "%s version %s (%s/%s)\n"
)

// -----------------------------------------------------------------------------
// UI Constants & Preferences
// -----------------------------------------------------------------------------

const (
	SettingsWindowWidth = 600

	// Preference Keys
	PrefCardDAVURL      = "carddav_url"
	PrefUsername        = "username"
	PrefLanguage        = "language"
	PrefInterval        = "refresh_interval_min"
	PrefServerPort      = "server_port"
	PrefSourceMode      = "source_mode"
	PrefLocalPath       = "local_path"
	PrefReminderEnabled = "reminder_enabled"
	PrefReminderValue   = "reminder_value"
	PrefReminderUnit    = "reminder_unit"
	PrefReminderDir     = "reminder_direction"
	PrefLastRun         = "last_run_version"
)

// SupportedLanguages defines the list of available UI languages (ISO 639-1).
var SupportedLanguages = []string{"en", "fr"}

// -----------------------------------------------------------------------------
// UI Contacts Window Constants
// -----------------------------------------------------------------------------

const (
	// Window Dimensions
	ContactsWinWidth  = 550 // Slightly wider to accommodate "Age -> Age"
	ContactsWinHeight = 400

	// Table Column IDs
	ColIDName = 0
	ColIDDate = 1
	ColIDAge  = 2

	// Table Layout
	ColWidthName = 250
	ColWidthDate = 120
	ColWidthAge  = 120 // Increased for transition format

	// Display Formats & Placeholders
	DateFormatDisplay = "2006-01-02"
	TablePlaceholder  = "Cell Content"
	AgeUnknown        = "-"
	AgeBirth          = "(birth)"
	LogMsgOpenWin     = "Opening Contacts Window"
	LogMsgSorted      = "Contacts sorted"

	// Sorting Indicators
	SortIconAsc  = " ▲"
	SortIconDesc = " ▼"
)

// -----------------------------------------------------------------------------
// Translation Keys (I18n)
// -----------------------------------------------------------------------------

const (
	TKeyWinTitle        = "win_title"
	TKeyWinContacts     = "win_contacts_title"
	TKeyMenuRefresh     = "menu_refresh"
	TKeyMenuSettings    = "menu_settings"
	TKeyTrayStatus      = "tray_status"      // Requires Count > 0
	TKeyTrayStatusZero  = "tray_status_zero" // Explicit key for 0
	TKeyNotifStart      = "notif_sync_start"
	TKeyNotifSuccess    = "notif_sync_success"
	TKeyNotifError      = "notif_err_sync"
	TKeyModeCardDAV     = "mode_carddav"
	TKeyModeLocal       = "mode_local"
	TKeyLblLanguage     = "lbl_language"
	TKeyHelpLanguage    = "help_language"
	TKeyLblMinutes      = "lbl_minutes_suffix"
	TKeyLblRefresh      = "lbl_refresh_interval"
	TKeyHelpInterval    = "help_interval"
	TKeyLblPort         = "lbl_server_port"
	TKeyHelpPort        = "help_port"
	TKeyLblGeneral      = "lbl_general"
	TKeyLblEnableRem    = "lbl_enable_reminders"
	TKeyUnitDays        = "unit_days"
	TKeyUnitHours       = "unit_hours"
	TKeyUnitMinutes     = "unit_minutes"
	TKeyDirBefore       = "dir_before"
	TKeyDirAfter        = "dir_after"
	TKeyLblNotif        = "lbl_notifications"
	TKeyBtnSave         = "btn_save"
	TKeyBtnCancel       = "btn_cancel"
	TKeyLblFooter       = "lbl_footer"
	TKeyBtnBrowse       = "btn_browse"
	TKeyLblURL          = "lbl_url"
	TKeyHelpURL         = "help_carddav_url"
	TKeyLblUser         = "lbl_user"
	TKeyLblPass         = "lbl_pass"
	TKeyLblSource       = "lbl_source"
	TKeyLblStartDay     = "lbl_start_of_day"
	TKeyEvtSummary      = "event_summary"       // Requires Name
	TKeyEvtSummaryAge   = "event_summary_age"   // Requires Name, Age
	TKeyEvtSummaryBirth = "event_summary_birth" // Requires Name (For age 0)

	// Column Headers & Formats
	TKeyColName    = "col_name"
	TKeyColDate    = "col_date"
	TKeyColAge     = "col_age"
	TKeyFormatDate = "format_date_short" // Date format pattern (e.g., "2006-01-02")
	TKeyAgeBirth   = "age_birth"         // Word for "Birth" / "Naissance" in list

	// Validation Errors (UI)
	TKeyErrPortReq   = "err_port_required"
	TKeyErrPortNum   = "err_port_number"
	TKeyErrPortRange = "err_port_range"
)

// -----------------------------------------------------------------------------
// Default Values & Business Logic
// -----------------------------------------------------------------------------

const (
	SourceModeWeb        = "web"
	SourceModeLocal      = "local"
	DefaultPort          = "18080"
	DefaultRefreshMin    = 60
	DefaultLanguage      = "en"
	DefaultLeapYear      = 2000 // Leap year fallback for dates like --02-29
	DefaultReminderValue = 1
	UIDSalt              = "go-birthday-v1-" // Salt for deterministic UID generation
	DisabledInterval     = 0
)

// ISO8601 Duration Components for Reminders
const (
	ISOPeriodPrefix   = "P"
	ISONegativePrefix = "-P"
	ISODay            = "D"
	ISOHour           = "H"
	ISOMinute         = "M"
)

// -----------------------------------------------------------------------------
// Standards: iCalendar & vCard
// -----------------------------------------------------------------------------

const (
	// iCal Properties
	ICalVersion   = "2.0"
	ICalProdid    = "-//Go Birthday//Engine//EN"
	ICalCalName   = "Birthdays"
	ICalMethod    = "PUBLISH"
	ICalScale     = "GREGORIAN"
	ICalComponent = "VALARM"
	ICalAction    = "DISPLAY"
	ICalDomain    = "gobirthday"

	// iCal/vCard Fields
	PropUID         = "UID"
	PropSummary     = "SUMMARY"
	PropDTStart     = "DTSTART"
	PropDTStamp     = "DTSTAMP"
	PropRefresh     = "REFRESH-INTERVAL"
	PropAction      = "ACTION"
	PropDescription = "DESCRIPTION"
	PropTrigger     = "TRIGGER"
	PropVersion     = "VERSION"
	PropProdid      = "PRODID"
	PropXWRCalName  = "X-WR-CALNAME"
	PropCalScale    = "CALSCALE"
	PropMethod      = "METHOD"

	VCardBDAY = "BDAY"
	VCardFN   = "FN"
	VCardN    = "N"

	DefaultICalRefresh = 1 * time.Hour
)

// -----------------------------------------------------------------------------
// Data Formats, Limits & File Extensions
// -----------------------------------------------------------------------------

const (
	// Date layouts used for parsing vCard BDAY fields
	DateFormatFullDash  = "2006-01-02"
	DateFormatFullBasic = "20060102"
	DateFormatRFC3339   = time.RFC3339
	DateFormatFullT     = "2006-01-02T15:04:05Z"
	DateFormatNoYearD   = "--01-02"
	DateFormatNoYearB   = "--0102"

	// Limits
	MinPort = 1
	MaxPort = 65535

	// UID Generation
	UIDHashLength   = 16
	FormatHashInput = "%s|%s|%s"
	FormatUID       = "%s-%d@%s"

	// File Extensions
	ExtVCF   = ".vcf"
	ExtVCard = ".vcard"
)

// -----------------------------------------------------------------------------
// Network & Timeouts
// -----------------------------------------------------------------------------

const (
	HTTPTimeout         = 30 * time.Second
	ShutdownTimeout     = 5 * time.Second
	ServerReadTimeout   = 10 * time.Second
	ServerWriteTimeout  = 30 * time.Second
	ServerIdleTimeout   = 60 * time.Second
	RetryAfterSeconds   = "10"
	AllowedMethods      = "GET, HEAD"
	MaxHTTPResponseSize = 256 * 1024 * 1024 // 256MB
	SchemeHTTP          = "http"
	SchemeHTTPS         = "https"
	RouteRoot           = "/"
	AddrSeparator       = ":"
)

// -----------------------------------------------------------------------------
// HTTP Headers & MIME Types
// -----------------------------------------------------------------------------

const (
	HeaderContentType     = "Content-Type"
	HeaderCacheControl    = "Cache-Control"
	HeaderETag            = "ETag"
	HeaderLastModified    = "Last-Modified"
	HeaderRetryAfter      = "Retry-After"
	HeaderAllow           = "Allow"
	HeaderXContentType    = "X-Content-Type-Options"
	HeaderUserAgent       = "User-Agent"
	HeaderIfNoneMatch     = "If-None-Match"
	HeaderIfModifiedSince = "If-Modified-Since"

	MimeTextCalendar    = "text/calendar; charset=utf-8"
	MimeNoSniff         = "nosniff"
	CacheControlPrivate = "private, no-cache"

	// FormatETag expects a string argument.
	FormatETag = `"%s"`
)

// -----------------------------------------------------------------------------
// Error Messages (Technical/Logs)
// -----------------------------------------------------------------------------

const (
	ErrLocalPathEmpty   = "configuration error: local path is empty"
	ErrWebURLEmpty      = "configuration error: web URL is empty"
	ErrFetcherMissing   = "internal error: network fetcher is not initialized"
	ErrModeUnsupport    = "configuration error: unsupported source mode"
	ErrServerStartup    = "server startup failed"
	ErrServerShutdown   = "server shutdown failed"
	ErrPortRequired     = "server port is required"
	ErrPortNumber       = "server port must be a number"
	ErrPortRange        = "server port must be between 1 and 65535"
	ErrInvalidURL       = "invalid URL structure"
	ErrProtocol         = "unsupported protocol scheme (http/https only)"
	ErrCtxCancelled     = "operation cancelled by context"
	ErrVCardParse       = "failed to parse vCard stream"
	ErrICalEncode       = "failed to encode iCalendar data"
	ErrDateParse        = "unable to parse date"
	ErrLogFile          = "failed to open log file"
	ErrCacheDir         = "could not determine user cache dir"
	ErrCreateDir        = "could not create app cache dir"
	ErrAppFailed        = "application failed unexpectedly"
	ErrWriteResp        = "failed to write response body"
	ErrLocalesAccess    = "failed to access embedded locales"
	ErrLocaleLoad       = "failed to load locale file"
	ErrTrayNotSupported = "system tray not supported on this platform/driver"
	ErrLocNotInit       = "localizer not initialized"
)

// -----------------------------------------------------------------------------
// HTTP Server Responses
// -----------------------------------------------------------------------------

const (
	HTTPMsgInitializing = "Calendar initializing, please try again shortly."
	HTTPMsgMethodNotAll = "Method Not Allowed"
	HTTPMsgInternalErr  = "Internal Server Error"
)

// -----------------------------------------------------------------------------
// Fallbacks & Defaults
// -----------------------------------------------------------------------------

const (
	FallbackSummary      = "Birthday: %s"
	FallbackSummaryAge   = "Birthday: %s (%d)"
	FallbackSummaryBirth = "Birthday: %s (birth)" // Lowercase fallback too
	FallbackTrayError    = "Go Birthday: Sync Error"
	FallbackTrayDefault  = "Go Birthday (%d today)"
	FallbackTrayLabel    = "Go Birthday"
	FallbackName         = "Unknown"

	// StubVCalendar is the minimal valid iCalendar object used when no events are found.
	// Using a constant avoids hardcoded magic strings in the engine logic.
	StubVCalendar = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:" + ICalProdid + "\r\nEND:VCALENDAR\r\n"

	TitleStartupError = "Startup Error"
	TitleSyncError    = "Sync Error"

	MsgPortBusy      = "Port %s is busy or unavailable."
	MsgSyncSuccess   = "Synchronization completed successfully."
	MsgSyncStarted   = "Synchronization started..."
	MsgSyncFailed    = "Synchronization failed. Check logs."
	MsgSyncReq       = "Sync requested"
	MsgWorkerStart   = "Background worker started"
	MsgWorkerStop    = "Worker stopping due to context cancellation"
	MsgUpdateSync    = "Updating sync interval"
	MsgAppStop       = "Application stopped gracefully"
	MsgCtxCancel     = "Context cancelled, shutting down UI"
	MsgSkippedCard   = "Skipping malformed vCard"
	MsgSkippedDate   = "Skipping invalid date format"
	MsgGenSuccess    = "Calendar generation successful"
	MsgAppStarting   = "Starting application"
	MsgServerListen  = "HTTP server listening"
	MsgServerStop    = "Shutting down HTTP server..."
	MsgCacheUpdated  = "Calendar cache updated"
	MsgLocaleSkip    = "Skipping non-locale file"
	MsgLocaleBadName = "Skipping malformed locale filename"
	MsgLocaleLoaded  = "Locale loaded successfully"
	MsgTransMissing  = "Missing translation key"
	MsgPassFail      = "Password retrieval failed (might be empty)"
	MsgLogWarning    = "Warning: %s at %s: %v\n"
	MsgBdayToday     = "Birthday found today"

	PlaceholderURL = "https://..."
)

// -----------------------------------------------------------------------------
// Reminder Units & Directions
// -----------------------------------------------------------------------------

const (
	UnitDays    = "d"
	UnitHours   = "h"
	UnitMinutes = "m"
	DirBefore   = "before"
	DirAfter    = "after"
)

// -----------------------------------------------------------------------------
// Structured Logging Keys (slog)
// -----------------------------------------------------------------------------

const (
	LogKeyComponent = "component"
	LogKeyError     = "error"
	LogKeyURL       = "url"
	LogKeyStatus    = "status_code"
	LogKeyFile      = "file"
	LogKeyLang      = "lang"
	LogKeyKey       = "key"
	LogKeyPort      = "port"
	LogKeyMode      = "mode"
	LogKeyInterval  = "interval"
	LogKeyOld       = "old"
	LogKeyNew       = "new"
	LogKeyUser      = "user"
	LogKeyTotal     = "total_cards"
	LogKeyFound     = "birthdays_found"
	LogKeyToday     = "birthdays_today"
	LogKeySizeBytes = "size_bytes"
	LogKeyETag      = "etag"
	LogKeyManual    = "manual"
	LogKeyValue     = "value"
	LogKeyStats     = "stats"
	LogKeySortCol   = "sort_column"
	LogKeySortAsc   = "sort_asc"
	LogKeyCount     = "count"
	LogKeyName      = "name"
	LogKeyDOB       = "date_of_birth"
	LogKeyDuration  = "duration_ms"

	// Startup Info Keys
	LogKeyBuild   = "build"
	LogKeyApp     = "app"
	LogKeyVersion = "version"
	LogKeyGoVer   = "go_version"
	LogKeyEnv     = "env"
	LogKeyOS      = "os"
	LogKeyArch    = "arch"
	LogKeyPID     = "pid"
)

// -----------------------------------------------------------------------------
// Log Components
// -----------------------------------------------------------------------------

const (
	CompUI      = "ui"
	CompUISet   = "ui_settings"
	CompEngine  = "engine"
	CompServer  = "server"
	CompFetcher = "fetcher"
	CompWorker  = "worker"
	CompMain    = "main"
	CompI18n    = "i18n"
)

// -----------------------------------------------------------------------------
// UI Layout Constants
// -----------------------------------------------------------------------------

const (
	LayoutColumnsDouble = 2
)
