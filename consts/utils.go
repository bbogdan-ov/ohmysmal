package consts

import (
	"time"
	"fmt"
)

func TimeAgo(t time.Time) string {
	dur := time.Since(t)

	switch {
	case dur < time.Minute:
		secs := int(dur.Seconds())
		if secs == 1 {
			return fmt.Sprintf("%d second ago", secs)
		} else {
			return fmt.Sprintf("%d seconds ago", secs)
		}
	case dur < time.Hour:
		mins := int(dur.Minutes())
		if mins == 1 {
			return fmt.Sprintf("%d minute ago", mins)
		} else {
			return fmt.Sprintf("%d minutes ago", mins)
		}
	case dur < 24*time.Hour:
		hours := int(dur.Hours())
		if hours == 1 {
			return fmt.Sprintf("%d hour ago", hours)
		} else {
			return fmt.Sprintf("%d hours ago", hours)
		}
	case dur < 30*24*time.Hour:
		days := int(dur.Hours() / 24)
		if days == 1 {
			return fmt.Sprintf("%d day ago", days)
		} else {
			return fmt.Sprintf("%d days ago", days)
		}
	case dur < 365*24*time.Hour:
		months := int(dur.Hours() / 24 / 30)
		if months == 1 {
			return fmt.Sprintf("%d month ago", months)
		} else {
			return fmt.Sprintf("%d months ago", months)
		}
	default:
		years := int(dur.Hours() / 24 / 365)
		if years == 1 {
			return fmt.Sprintf("%d year ago", years)
		} else {
			return fmt.Sprintf("%d years ago", years)
		}
	}
}
