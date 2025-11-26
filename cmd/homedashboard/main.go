package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"html/template"
	"image/png"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/chromedp/chromedp"
)

//go:embed templates/*
var templateFS embed.FS

var (
	quoteTpl     *template.Template
	transportTpl *template.Template
	weatherTpl   *template.Template
	calendarTpl  *template.Template
	rootCtx      context.Context
	browser      context.Context
)

func main() {
	quoteTpl = template.Must(template.ParseFS(templateFS, "templates/quote.html"))
	transportTpl = template.Must(template.ParseFS(templateFS, "templates/transport.html"))
	weatherTpl = template.Must(template.ParseFS(templateFS, "templates/weather.html"))
	calendarTpl = template.Must(template.ParseFS(templateFS, "templates/calendar.html"))
	rootCtx, _ = chromedp.NewExecAllocator(context.Background(),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.WindowSize(800, 480),
	)
	browser, _ = chromedp.NewContext(rootCtx)
	loadAllImages(context.Background())
	startBackgroundRenderer(browser)

	http.HandleFunc("/dashboard.bmp", handleDashboardBMP)
	http.HandleFunc("/quote.bmp", handleQuoteBMP)
	http.HandleFunc("/transport.bmp", handleTransportBMP)
	http.HandleFunc("/weather.bmp", handleWeatherBMP)
	http.HandleFunc("/photo.bmp", handlePhotoBMP)
	http.HandleFunc("/stocks.bmp", handleStocksBMP)
	http.HandleFunc("/calendar.bmp", handleCalendarBMP)

	log.Println("Listening on https://<PI-IP>:8443 ...")
	log.Fatal(http.ListenAndServe(":8443", nil))
}

var lastPage = 0

func handleDashboardBMP(w http.ResponseWriter, r *http.Request) {
	from := parseClock("07:15")
	to := parseClock("07:40")
	now := nowInMinutes()
	if now.Between(from, to) {
		handleTransportBMP(w, r)
		return
	} else {
		if lastPage == 0 {
			handleWeatherBMP(w, r)
		} else if lastPage == 1 {
			handleQuoteBMP(w, r)
		} else if lastPage == 2 {
			handleStocksBMP(w, r)
		} else if lastPage == 3 {
			handleCalendarBMP(w, r)
		}
		lastPage = (lastPage + 1) % 4
	}
}

func serveCachedImage(w http.ResponseWriter, r *http.Request, cache *CachedImage) {
	data, updatedAt := cache.Get()
	if data == nil {
		http.Error(w, "image not ready", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "image/bmp")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Last-Modified", updatedAt.Format(http.TimeFormat))
	if _, err := w.Write(data); err != nil {
		log.Println("write image error:", err)
	}
}

func htmlToBMP(html string) ([]byte, error) {
	dataURL := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html))

	tabCtx, tabCancel := chromedp.NewContext(browser)
	defer tabCancel()

	ctx, cancel := context.WithTimeout(tabCtx, 10*time.Second)
	defer cancel()

	var p []byte
	err := chromedp.Run(ctx,
		chromedp.Navigate(dataURL),
		chromedp.WaitReady("body"),
		chromedp.EmulateViewport(800, 480),
		chromedp.CaptureScreenshot(&p),
	)
	if err != nil {
		log.Println("chromedp run:", err)
		return nil, err
	}

	img, err := png.Decode(bytes.NewReader(p))
	if err != nil {
		log.Println("png decode:", err)
		return nil, err
	}

	bmp, err := encode1bppBMP(ditherFloydSteinbergHybrid(img, false))
	if err != nil {
		log.Println("bmp encode:", err)
		return nil, err
	}

	return bmp, nil
}

type TimeOfDay int // минуты с полуночи

func parseClock(hhmm string) TimeOfDay {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		panic(err) // можно завернуть в log.Fatalf или вернуть ошибку наверх
	}
	return TimeOfDay(t.Hour()*60 + t.Minute())
}

func nowInMinutes() TimeOfDay {
	now := time.Now()
	return TimeOfDay(now.Hour()*60 + now.Minute())
}

func (t TimeOfDay) Before(other TimeOfDay) bool { return t < other }
func (t TimeOfDay) After(other TimeOfDay) bool  { return t > other }
func (t TimeOfDay) Between(from, to TimeOfDay) bool {
	return t >= from && t < to
}
