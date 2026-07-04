package generator

import (
	"fmt"
	"math/rand/v2"

	"synthgraph/internal/schema"
)

// timestampGenerator generates random timestamp strings.
type timestampGenerator struct{}

func (generator timestampGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	minTime := int64(1577836800) // 2020-01-01
	maxTime := int64(1798761600) // 2026-12-31
	rangeSize := maxTime - minTime + 1
	unixTime := minTime + rng.Int64N(rangeSize)
	return formatTimestamp(unixTime), nil
}

// formatTimestamp formats a Unix timestamp as an ISO 8601 string.
// Simple implementation without time.Time (avoids timezone complications).
func formatTimestamp(unixTime int64) string {
	secondsInDay := int64(86400)
	days := unixTime / secondsInDay
	timeOfDay := unixTime % secondsInDay

	hours := timeOfDay / 3600
	minutes := (timeOfDay % 3600) / 60
	seconds := timeOfDay % 60

	year, month, day := daysToDate(days)
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", year, month, day, hours, minutes, seconds)
}

// daysToDate converts a day count (since 2020-01-01) to year/month/day.
func daysToDate(days int64) (int, int, int) {
	daysInMonth := []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	isLeap := func(year int) bool {
		return year%4 == 0 && (year%100 != 0 || year%400 == 0)
	}

	year := 2020
	remaining := days

	for {
		yearDays := 365
		if isLeap(year) {
			yearDays = 366
		}
		if remaining < int64(yearDays) {
			break
		}
		remaining -= int64(yearDays)
		year++
	}

	dim := make([]int, 12)
	copy(dim, daysInMonth)
	if isLeap(year) {
		dim[1] = 29
	}

	month := 1
	for ; month <= 12; month++ {
		if remaining < int64(dim[month-1]) {
			break
		}
		remaining -= int64(dim[month-1])
	}

	day := int(remaining) + 1
	if month > 12 {
		month = 12
		day = 31
	}
	if day < 1 {
		day = 1
	}

	return year, month, day
}
