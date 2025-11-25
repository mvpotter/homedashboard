package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	ics "github.com/arran4/golang-ical"
)

type CalendarEvent struct {
	Start    time.Time
	End      time.Time
	Summary  string
	Location string
	AllDay   bool
}

type calendarPageData struct {
	Range   string
	Columns [][]CalendarEvent
}

func loadWeekEventsFromICS(periodStart time.Time, periodEnd time.Time) ([]CalendarEvent, error) {
	resp, err := http.Get("https://calendar.google.com/calendar/ical/family12077548807296936472%40group.calendar.google.com/private-3e7a703aaa3f1462d0d9c2bf7faa0a9a/basic.ics")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	cal, err := ics.ParseCalendar(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var events []CalendarEvent
	loc, _ := time.LoadLocation("Europe/Berlin")
	for _, e := range cal.Events() {
		start, _ := e.GetStartAt()
		end, _ := e.GetEndAt()

		if start.IsZero() {
			continue
		}

		// фильтруем только события в ближайшую неделю
		if end.Before(periodStart) || start.After(periodEnd) {
			continue
		}

		summary := e.GetProperty(ics.ComponentPropertySummary)
		location := e.GetProperty(ics.ComponentPropertyLocation)

		ev := CalendarEvent{
			Start:    start.In(loc),
			End:      end.In(loc),
			Summary:  "",
			Location: "",
			AllDay:   isAllDay(e),
		}
		if summary != nil {
			ev.Summary = summary.Value
		}
		if location != nil {
			ev.Location = location.Value
		}

		events = append(events, ev)
	}

	// сортируем по времени начала
	sort.Slice(events, func(i, j int) bool {
		return events[i].Start.Before(events[j].Start)
	})

	return events, nil
}

func isAllDay(e *ics.VEvent) bool {
	p := e.GetProperty(ics.ComponentPropertyDtStart)
	if p == nil {
		return false
	}

	v := p.Value
	if v == "DATE" {
		return true
	}

	return false
}

func renderCalendarBMP(ctx context.Context) ([]byte, error) {
	now := time.Now()
	loc := now.Location()
	periodStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	periodEnd := periodStart.AddDate(0, 1, 0)

	events, err := loadWeekEventsFromICS(periodStart, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("unable load events: %w", err)
	}

	data := calendarPageData{
		Range:   periodStart.Format("02.01.") + " – " + periodEnd.AddDate(0, 0, -1).Format("02.01."),
		Columns: buildColumnsFixed(events, 3, 8),
	}

	var buf bytes.Buffer
	if err := calendarTpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("unable render calendar template: %w", err)
	}
	htmlStr := buf.String()

	png, err := htmlToBMP(htmlStr)
	if err != nil {
		return nil, fmt.Errorf("unable render calendar bmp: %w", err)
	}

	return png, nil
}

func buildColumnsFixed(events []CalendarEvent, colCount, perCol int) [][]CalendarEvent {
	cols := make([][]CalendarEvent, colCount)
	if len(events) == 0 {
		return cols
	}

	colIdx := 0
	countInCol := 0

	for _, ev := range events {
		cols[colIdx] = append(cols[colIdx], ev)
		countInCol++

		if countInCol >= perCol {
			// переходим к следующей колонке
			colIdx++
			countInCol = 0
			if colIdx >= colCount {
				// если колонок больше нет — всё остальное тоже в последнюю
				colIdx = colCount - 1
			}
		}
	}

	return cols
}

func handleCalendarBMP(w http.ResponseWriter, r *http.Request) {
	serveCachedImage(w, r, &calendarCache)
}
