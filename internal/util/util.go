package util

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

var _loc *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Singapore")
	if err != nil {
		panic(err)
	}
	_loc = loc
}

func MarshaltoString(ctx context.Context, obj any) string {
	res, _ := json.Marshal(obj)
	return string(res)
}

// timeStr is in 24 hour format 15:04
func GetNextScheduledTime(day int, timeStr string) (time.Time, error) {
	wantTime, err := time.ParseInLocation("15:04", timeStr, _loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time format: %w", err)
	}

	now := CurrentTimeInSG()

	// Start with today, but set time to the requested hour and minute
	nextTime := time.Date(now.Year(), now.Month(), now.Day(),
		wantTime.Hour(), wantTime.Minute(), 0, 0, CurrentTimeInSG().Location())

	// Compute how many days until the target day
	daysUntil := (day - int(now.Weekday()) + 7) % 7
	if daysUntil == 0 && !nextTime.After(now) {
		// It's today but time has passed, so go to next week
		daysUntil = 7
	}
	nextTime = nextTime.AddDate(0, 0, daysUntil)

	return nextTime, nil
}

func CurrentTimeInSG() time.Time {
	return time.Now().In(_loc)
}
