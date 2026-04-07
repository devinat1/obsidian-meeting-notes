// services/calendar-service/internal/calendar/client.go
package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"golang.org/x/oauth2"
	calendarapi "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

type FetchEventsParams struct {
	TokenSource oauth2.TokenSource
	TimeMin     time.Time
	TimeMax     time.Time
}

type CalendarEvent struct {
	CalendarEventID string
	Title           string
	StartTime       time.Time
	EndTime         time.Time
	MeetingURL      string
	Attendees       json.RawMessage
}

// FetchUpcomingEvents fetches Google Calendar events with video meeting links within the given time range.
func (c *Client) FetchUpcomingEvents(ctx context.Context, params FetchEventsParams) ([]CalendarEvent, error) {
	svc, err := calendarapi.NewService(ctx, option.WithTokenSource(params.TokenSource))
	if err != nil {
		return nil, fmt.Errorf("Failed to create calendar service: %w.", err)
	}

	events, err := svc.Events.List("primary").
		TimeMin(params.TimeMin.Format(time.RFC3339)).
		TimeMax(params.TimeMax.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		ShowDeleted(true).
		Fields("items(id,summary,start,end,conferenceData,description,location,attendees,status)").
		Do()
	if err != nil {
		return nil, fmt.Errorf("Failed to list calendar events: %w.", err)
	}

	result := []CalendarEvent{}
	for _, event := range events.Items {
		// Skip cancelled events (handled separately by poller for cancellation detection).
		if event.Status == "cancelled" {
			continue
		}

		meetingURL := ExtractMeetingURL(event)
		if meetingURL == "" {
			continue
		}

		startTime, err := parseEventTime(event.Start)
		if err != nil {
			slog.Warn("Failed to parse event start time, skipping.", "event_id", event.Id, "error", err)
			continue
		}

		endTime, err := parseEventTime(event.End)
		if err != nil {
			slog.Warn("Failed to parse event end time, skipping.", "event_id", event.Id, "error", err)
			continue
		}

		attendees := extractAttendees(event.Attendees)
		attendeesJSON, err := json.Marshal(attendees)
		if err != nil {
			slog.Warn("Failed to marshal attendees.", "event_id", event.Id, "error", err)
			attendeesJSON = []byte("[]")
		}

		result = append(result, CalendarEvent{
			CalendarEventID: event.Id,
			Title:           event.Summary,
			StartTime:       startTime,
			EndTime:         endTime,
			MeetingURL:      meetingURL,
			Attendees:       json.RawMessage(attendeesJSON),
		})
	}

	return result, nil
}

// FetchDeletedEventIDs returns event IDs that have been cancelled since we last polled.
func (c *Client) FetchDeletedEventIDs(ctx context.Context, params FetchEventsParams) ([]string, error) {
	svc, err := calendarapi.NewService(ctx, option.WithTokenSource(params.TokenSource))
	if err != nil {
		return nil, fmt.Errorf("Failed to create calendar service: %w.", err)
	}

	events, err := svc.Events.List("primary").
		TimeMin(params.TimeMin.Format(time.RFC3339)).
		TimeMax(params.TimeMax.Format(time.RFC3339)).
		SingleEvents(true).
		ShowDeleted(true).
		Fields("items(id,status)").
		Do()
	if err != nil {
		return nil, fmt.Errorf("Failed to list calendar events for deletion check: %w.", err)
	}

	deletedIDs := []string{}
	for _, event := range events.Items {
		if event.Status == "cancelled" {
			deletedIDs = append(deletedIDs, event.Id)
		}
	}

	return deletedIDs, nil
}

func parseEventTime(eventDateTime *calendarapi.EventDateTime) (time.Time, error) {
	if eventDateTime == nil {
		return time.Time{}, fmt.Errorf("Event date/time is nil.")
	}

	// Prefer DateTime (specific time) over Date (all-day event).
	if eventDateTime.DateTime != "" {
		return time.Parse(time.RFC3339, eventDateTime.DateTime)
	}

	if eventDateTime.Date != "" {
		return time.Parse("2006-01-02", eventDateTime.Date)
	}

	return time.Time{}, fmt.Errorf("Event has no date or datetime.")
}

func extractAttendees(gcalAttendees []*calendarapi.EventAttendee) []model.Attendee {
	attendees := []model.Attendee{}
	for _, a := range gcalAttendees {
		attendees = append(attendees, model.Attendee{
			Email:       a.Email,
			DisplayName: a.DisplayName,
			Self:        a.Self,
		})
	}
	return attendees
}
