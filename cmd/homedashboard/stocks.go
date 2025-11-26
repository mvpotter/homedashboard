package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/bmp"
	"golang.org/x/image/font"
)

//go:embed assets/JetBrainsMono-Bold.ttf
var fontCustom []byte

type yfResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
			} `json:"meta"`

			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`

		Error any `json:"error"`
	} `json:"chart"`
}

func loadVWCEYear() ([]time.Time, []float64, error) {
	url := "https://query1.finance.yahoo.com/v7/finance/chart/VWCE.DE" +
		"?range=1y&interval=1d&includePrePost=false"

	req, _ := http.NewRequest("GET", url, nil)
	// без User-Agent часто шлёт HTML-страницы, а не JSON
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; reTerminalBot/1.0)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read body: %w", err)
	}

	var yf yfResponse
	if err := json.Unmarshal(body, &yf); err != nil {
		return nil, nil, fmt.Errorf("decode error: %w\nbody: %s", err, string(body))
	}

	if len(yf.Chart.Result) == 0 || yf.Chart.Error != nil {
		return nil, nil, fmt.Errorf("yahoo chart error: %#v", yf.Chart.Error)
	}

	r := yf.Chart.Result[0]
	ts := r.Timestamp
	closeSlice := r.Indicators.Quote[0].Close

	if len(ts) != len(closeSlice) {
		return nil, nil, fmt.Errorf("len(timestamp) != len(close)")
	}

	dates := make([]time.Time, 0, len(ts))
	prices := make([]float64, 0, len(ts))

	for i := range ts {
		p := closeSlice[i]
		if p <= 0 {
			continue
		}
		dates = append(dates, time.Unix(ts[i], 0))
		prices = append(prices, p)
	}

	if len(prices) == 0 {
		return nil, nil, fmt.Errorf("no valid prices")
	}

	return dates, prices, nil
}

func renderVWCEChart(dates []time.Time, prices []float64) image.Image {
	const W, H = 800, 480

	dc := gg.NewContext(W, H)

	// фон
	dc.SetRGB(0, 0, 0)
	dc.Clear()

	const left = 70.0
	const right = 20.0
	const top = 90.0
	const bottom = 80.0

	n := len(prices)
	if n == 0 {
		return dc.Image()
	}

	// min/max
	minP, maxP := prices[0], prices[0]
	for _, p := range prices {
		if p < minP {
			minP = p
		}
		if p > maxP {
			maxP = p
		}
	}
	if maxP == minP {
		maxP += 1
	}

	first := prices[0]
	last := prices[n-1]
	changePct := (last/first - 1.0) * 100

	width := float64(W) - left - right
	height := float64(H) - top - bottom

	dc.SetRGB(1, 1, 1)

	// ===== Заголовок =====
	dc.SetFontFace(boldFace(12))
	title := fmt.Sprintf("VWCE  %.2f €  (%+.1f%%)", last, changePct)
	dc.DrawStringAnchored(title, W/2, 42, 0.5, 0.5)

	// ===== Даты =====
	dc.SetFontFace(boldFace(12))
	dateRange := fmt.Sprintf("%s — %s",
		dates[0].Format("02.01.2006"),
		dates[n-1].Format("02.01.2006"),
	)
	dc.DrawStringAnchored(dateRange, W/2, 68, 0.5, 0.5)

	// ===== Ось Y: 3 деления =====
	dc.SetFontFace(boldFace(10))
	for i := 0; i <= 2; i++ {
		val := minP + (maxP-minP)*float64(i)/2.0
		y := H - bottom - (float64(i)/2.0)*(H-bottom-top)

		dc.SetLineWidth(0.7)
		dc.DrawLine(left, y, W-right, y)
		dc.Stroke()

		label := fmt.Sprintf("%.0f", val)
		dc.DrawStringAnchored(label, left-10, y, 1.0, 0.5)
	}

	// ===== Месяца по X: каждый, формата 11.25 =====
	type monthKey struct {
		year  int
		month time.Month
	}
	type monthTick struct {
		idx int
		t   time.Time
	}

	seen := make(map[monthKey]bool)
	var ticks []monthTick
	for i, t := range dates {
		k := monthKey{year: t.Year(), month: t.Month()}
		if !seen[k] {
			seen[k] = true
			ticks = append(ticks, monthTick{idx: i, t: t})
		}
	}

	// если вдруг больше 12 — берём последние 12
	if len(ticks) > 12 {
		ticks = ticks[len(ticks)-12:]
	}

	dc.SetFontFace(boldFace(10))
	for _, tick := range ticks {
		x := left + (float64(tick.idx)/float64(n-1))*width

		dc.SetLineWidth(0.7)
		dc.DrawLine(x, top, x, H-bottom)
		dc.Stroke()

		label := tick.t.Format("01.06") // "11.25"
		dc.DrawStringAnchored(label, x, H-bottom+28, 0.5, 0.0)
	}

	// ===== Линия графика =====
	dc.SetLineWidth(3)

	for i := 1; i < n; i++ {
		x1 := left + (float64(i-1)/float64(n-1))*width
		y1 := H - bottom - ((prices[i-1]-minP)/(maxP-minP))*height
		x2 := left + (float64(i)/float64(n-1))*width
		y2 := H - bottom - ((prices[i]-minP)/(maxP-minP))*height

		dc.DrawLine(x1, y1, x2, y2)
		dc.Stroke()
	}

	dc.DrawCircle(
		left+(float64(n-1)/float64(n-1))*width,
		H-bottom-((last-minP)/(maxP-minP))*height,
		4,
	)
	dc.Fill()

	return dc.Image()
}

func boldFace(size float64) font.Face {
	f, err := truetype.Parse(fontCustom)
	if err != nil {
		log.Fatalf("failed to parse font: %v", err)
	}
	return truetype.NewFace(f, &truetype.Options{
		Size:    size,
		DPI:     96,
		Hinting: font.HintingFull,
	})
}

func renderVWCEBMP(ctx context.Context) ([]byte, error) {
	dates, prices, err := loadVWCEYear()
	if err != nil {
		log.Println("loadVWCEYear error:", err)
		return nil, err
	}

	img := renderVWCEChart(dates, prices)
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func handleStocksBMP(w http.ResponseWriter, r *http.Request) {
	serveCachedImage(w, r, &stocksCache)
}
