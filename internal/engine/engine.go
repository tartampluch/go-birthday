package engine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-vcard"
	"github.com/tartampluch/go-birthday/internal/config"
)

// SyncConfig contains all parameters required to perform a synchronization.
type SyncConfig struct {
	Mode            string // config.SourceModeLocal or config.SourceModeWeb
	LocalPath       string // Absolute path to the .vcf file
	WebURL          string // CardDAV or WebDAV URL
	WebUser         string // HTTP Basic Auth Username
	WebPass         string // HTTP Basic Auth Password
	ReminderTrigger string // ISO8601 duration string (e.g., "-P1D")
}

// Generator is the core service responsible for fetching and converting data.
type Generator struct {
	Clock   Clock        // Interface for time mocking.
	Fetcher VCardFetcher // Interface for network abstraction.

	// FormatSummary allows the UI to inject localized strings into the logic layer.
	FormatSummary func(name string, age int, yearKnown bool) string
}

// RunSync executes the fetching, parsing, and generation pipeline.
// It returns the ICS data, the list of contacts, the count of birthdays today, and any error.
func (g *Generator) RunSync(ctx context.Context, cfg SyncConfig) ([]byte, []BirthdayEntry, int, error) {
	start := time.Now()
	log := slog.With(
		config.LogKeyComponent, config.CompEngine,
		config.LogKeyMode, cfg.Mode,
	)
	log.InfoContext(ctx, config.MsgSyncStarted)

	// 1. Acquire Data Stream
	reader, err := g.acquireStream(ctx, cfg)
	if err != nil {
		// If context error occurred during acquisition, return it directly.
		if ctx.Err() != nil {
			return nil, nil, 0, ctx.Err()
		}
		return nil, nil, 0, fmt.Errorf("%s: %w", config.ErrVCardParse, err)
	}
	// Best effort close. Errors in Close() for read-only files are rarely actionable here.
	defer func() { _ = reader.Close() }()

	// Check for early cancellation before processing
	if err := ctx.Err(); err != nil {
		return nil, nil, 0, err
	}

	// 2. Process Data
	ics, contacts, count, err := g.generateCalendar(ctx, reader, cfg.ReminderTrigger)

	// Log performance metric
	if err == nil {
		log.Debug("Sync finished", config.LogKeyDuration, time.Since(start).Milliseconds())
	}
	return ics, contacts, count, err
}

// acquireStream opens the appropriate data source based on configuration.
func (g *Generator) acquireStream(ctx context.Context, cfg SyncConfig) (io.ReadCloser, error) {
	switch cfg.Mode {
	case config.SourceModeLocal:
		if cfg.LocalPath == "" {
			return nil, errors.New(config.ErrLocalPathEmpty)
		}
		return os.Open(cfg.LocalPath)
	case config.SourceModeWeb:
		if cfg.WebURL == "" {
			return nil, errors.New(config.ErrWebURLEmpty)
		}
		if g.Fetcher == nil {
			return nil, errors.New(config.ErrFetcherMissing)
		}
		return g.Fetcher.Fetch(ctx, cfg.WebURL, cfg.WebUser, cfg.WebPass)
	default:
		return nil, fmt.Errorf("%s: %q", config.ErrModeUnsupport, cfg.Mode)
	}
}

// generateCalendar parses the vCard stream and constructs the iCalendar object.
// It also builds the BirthdayEntry list for the UI.
func (g *Generator) generateCalendar(ctx context.Context, r io.Reader, reminderTrigger string) ([]byte, []BirthdayEntry, int, error) {
	cal := ical.NewCalendar()

	// Set standard iCalendar headers
	cal.Props.SetText(config.PropVersion, config.ICalVersion)
	cal.Props.SetText(config.PropProdid, config.ICalProdid)
	cal.Props.SetText(config.PropXWRCalName, config.ICalCalName)
	cal.Props.SetText(config.PropCalScale, config.ICalScale)
	cal.Props.SetText(config.PropMethod, config.ICalMethod)

	// RFC 7986: Suggest a refresh interval (Standardized in config)
	refreshProp := ical.NewProp(config.PropRefresh)
	refreshProp.SetDuration(config.DefaultICalRefresh)
	cal.Props.Set(refreshProp)

	// CRITICAL FIX: Use Local time for logic, convert to UTC only for ICS stamping.
	// Birthdays are defined by the local calendar date of the person, not an absolute UTC timestamp.
	// If it is June 15th in Tokyo, it is the user's birthday, even if it is still June 14th in UTC.
	now := g.Clock.Now()
	dtStampProp := ical.NewProp(config.PropDTStamp)
	dtStampProp.SetDateTime(now.UTC())

	decoder := vcard.NewDecoder(r)
	stats := struct{ processed, withBday, today int }{0, 0, 0}
	var contacts []BirthdayEntry

	for {
		if ctx.Err() != nil {
			return nil, nil, 0, ctx.Err()
		}

		card, err := decoder.Decode()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			// Log error but continue to next card to maximize data recovery
			slog.Warn(config.MsgSkippedCard,
				config.LogKeyComponent, config.CompEngine,
				config.LogKeyError, err)
			continue
		}

		stats.processed++
		bday := card.Get(config.VCardBDAY)
		if bday == nil || bday.Value == "" {
			continue
		}

		birthDate, yearKnown, err := parseDate(bday.Value)
		if err != nil {
			slog.Debug(config.MsgSkippedDate,
				config.LogKeyComponent, config.CompEngine,
				config.LogKeyValue, bday.Value)
			continue
		}
		stats.withBday++

		// Name Strategy: FN (Formatted) > N (Structured) > Fallback
		name := config.FallbackName
		if fn := card.Get(config.VCardFN); fn != nil {
			name = fn.Value
		} else if n := card.Get(config.VCardN); n != nil {
			name = n.Value
		}

		// --- Logic 1: Prepare UI Data (Contact List) ---

		// Deterministic UID generation for stability across refreshes
		input := fmt.Sprintf(config.FormatHashInput, name, birthDate.Format(time.RFC3339), config.UIDSalt)
		hash := sha256.Sum256([]byte(input))
		uidBase := fmt.Sprintf("%x", hash[:config.UIDHashLength])

		// Calculate when the birthday occurs next (for sorting purposes)
		nextOcc, ageNext := calculateNextOccurrence(now, birthDate, yearKnown)

		contacts = append(contacts, BirthdayEntry{
			UID:            uidBase,
			Name:           name,
			DateOfBirth:    birthDate,
			YearKnown:      yearKnown,
			NextOccurrence: nextOcc,
			AgeNext:        ageNext,
		})

		// --- Logic 2: Prepare ICS Events (Calendar) ---

		events, isToday := g.createEvents(name, birthDate, yearKnown, reminderTrigger, now, uidBase)
		if isToday {
			stats.today++
			// DEBUG: Log explicitly WHO is triggering "today" for verification
			slog.Info(config.MsgBdayToday,
				config.LogKeyComponent, config.CompEngine,
				config.LogKeyName, name,
				config.LogKeyDOB, birthDate.Format(config.DateFormatFullDash))
		}

		for _, e := range events {
			e.Props.Set(dtStampProp)
			cal.Children = append(cal.Children, e.Component)
		}
	}

	// Handle case where no events are found.
	if len(cal.Children) == 0 {
		var buf bytes.Buffer
		// Use the constant stub to ensure a valid VCALENDAR is returned even if empty.
		// This prevents clients from flagging the feed as invalid.
		fmt.Fprintf(&buf, config.StubVCalendar)

		g.logSuccess(stats)
		return buf.Bytes(), contacts, 0, nil
	}

	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return nil, nil, 0, fmt.Errorf("%s: %w", config.ErrICalEncode, err)
	}

	g.logSuccess(stats)
	return buf.Bytes(), contacts, stats.today, nil
}

// logSuccess logs the final statistics of the generation process.
func (g *Generator) logSuccess(stats struct{ processed, withBday, today int }) {
	slog.Info(config.MsgGenSuccess,
		config.LogKeyComponent, config.CompEngine,
		slog.Group(config.LogKeyStats,
			slog.Int(config.LogKeyTotal, stats.processed),
			slog.Int(config.LogKeyFound, stats.withBday),
			slog.Int(config.LogKeyToday, stats.today),
		),
	)
}

// calculateNextOccurrence determines the next birthday date relative to 'now'.
// This is used primarily for sorting the contact list.
func calculateNextOccurrence(now time.Time, birthDate time.Time, yearKnown bool) (time.Time, int) {
	currentYear := now.Year()
	// Fix: Use the location of 'now' to ensure timezone consistency
	loc := now.Location()

	// Create a candidate date for the current year.
	// Go's time.Date normalizes Feb 29 to March 1st if currentYear is not a leap year.
	candidate := time.Date(currentYear, birthDate.Month(), birthDate.Day(), 0, 0, 0, 0, loc)

	// Check if this candidate date is in the past (strictly before the start of today).
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	if candidate.Before(todayStart) {
		// Birthday has already passed this year, next one is next year.
		candidate = time.Date(currentYear+1, birthDate.Month(), birthDate.Day(), 0, 0, 0, 0, loc)
	}

	ageNext := 0
	if yearKnown {
		ageNext = candidate.Year() - birthDate.Year()
	}

	return candidate, ageNext
}

// createEvents generates calendar events for CurrentYear-1, CurrentYear, and CurrentYear+1.
// It ensures no events are created before the person is born.
func (g *Generator) createEvents(name string, birthDate time.Time, yearKnown bool, reminderTrigger string, now time.Time, uidBase string) ([]*ical.Event, bool) {
	currentYear := now.Year()
	// Requirement: Generate for Previous Year, Current Year, Next Year (3 years total)
	// This ensures that when a user scrolls back or forward in their calendar app,
	// the events are present without needing an immediate re-sync.
	targetYears := []int{currentYear - 1, currentYear, currentYear + 1}
	loc := now.Location()

	var events []*ical.Event
	isToday := false

	todayYear, todayMonth, todayDay := now.Date()

	for _, y := range targetYears {
		// Guard: Do not generate an event if the person is not born yet in year 'y'.
		if yearKnown && y < birthDate.Year() {
			continue
		}

		event := ical.NewEvent()
		event.Props.SetText(config.PropUID, fmt.Sprintf(config.FormatUID, uidBase, y, config.ICalDomain))

		age := 0
		if yearKnown {
			age = y - birthDate.Year()
		}

		// Generate localized summary
		summary := fmt.Sprintf(config.FallbackSummary, name)
		if g.FormatSummary != nil {
			// Pass 'age' to formatter. If age is 0 and year is known, formatter should handle "(Birth)".
			summary = g.FormatSummary(name, age, yearKnown && age >= 0)
		}
		event.Props.SetText(config.PropSummary, summary)

		// Date Normalization
		eventDate := time.Date(y, birthDate.Month(), birthDate.Day(), 0, 0, 0, 0, loc)

		if y == todayYear && eventDate.Month() == todayMonth && eventDate.Day() == todayDay {
			isToday = true
		}

		dtStartProp := ical.NewProp(config.PropDTStart)
		// Set date (value=DATE). Timezone is less relevant for full-day events but consistency helps.
		dtStartProp.SetDate(eventDate)
		event.Props.Set(dtStartProp)

		if reminderTrigger != "" {
			addAlarm(event, reminderTrigger, summary)
		}

		events = append(events, event)
	}
	return events, isToday
}

// addAlarm appends a DISPLAY alarm (notification) to the event.
func addAlarm(event *ical.Event, trigger, description string) {
	alarm := ical.NewComponent(config.ICalComponent)
	alarm.Props.SetText(config.PropAction, config.ICalAction)
	alarm.Props.SetText(config.PropDescription, description)

	// Set trigger manually to avoid "VALUE=TEXT" param
	triggerProp := ical.NewProp(config.PropTrigger)
	triggerProp.Value = trigger
	alarm.Props.Set(triggerProp)

	event.Children = append(event.Children, alarm)
}

// parseDate handles various vCard date formats.
func parseDate(value string) (time.Time, bool, error) {
	// Full dates (Year known)
	formatsWithYear := []string{
		config.DateFormatFullDash,
		config.DateFormatFullBasic,
		config.DateFormatRFC3339,
		config.DateFormatFullT,
	}

	for _, f := range formatsWithYear {
		if t, err := time.Parse(f, value); err == nil {
			return t, true, nil
		}
	}

	// Truncated dates (Year unknown) - vCard specific
	// Safe leap year fallback
	formatsWithoutYear := []string{config.DateFormatNoYearD, config.DateFormatNoYearB}
	for _, f := range formatsWithoutYear {
		if t, err := time.Parse(f, value); err == nil {
			safeDate := time.Date(config.DefaultLeapYear, t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			return safeDate, false, nil
		}
	}

	return time.Time{}, false, errors.New(config.ErrDateParse)
}
