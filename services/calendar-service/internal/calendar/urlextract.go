// services/calendar-service/internal/calendar/urlextract.go
package calendar

import (
	"regexp"
	"strings"

	calendarapi "google.golang.org/api/calendar/v3"
)

var (
	zoomURLPattern  = regexp.MustCompile(`https?://[\w.-]*zoom\.us/j/\d+[^\s"<>]*`)
	teamsURLPattern = regexp.MustCompile(`https?://teams\.microsoft\.com/l/meetup-join/[^\s"<>]+`)
)

// ExtractMeetingURL extracts a video meeting URL from a Google Calendar event.
// Priority: Google Meet conferenceData > Zoom regex > Teams regex.
// Returns empty string if no meeting URL is found.
func ExtractMeetingURL(event *calendarapi.Event) string {
	// Google Meet: check conferenceData entry points.
	if event.ConferenceData != nil {
		for _, ep := range event.ConferenceData.EntryPoints {
			if ep.EntryPointType == "video" && ep.Uri != "" {
				return ep.Uri
			}
		}
	}

	// Zoom and Teams: regex on description and location.
	searchText := strings.Join([]string{event.Description, event.Location}, " ")

	if match := zoomURLPattern.FindString(searchText); match != "" {
		return match
	}

	if match := teamsURLPattern.FindString(searchText); match != "" {
		return match
	}

	return ""
}
