package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"
)

//
// ---------- RAW API RESPONSE ----------
//

type openMeteoResponse struct {
	CurrentWeather struct {
		Temperature2m       float64 `json:"temperature_2m"`
		WeatherCode         int     `json:"weather_code"`
		RelativeHumidity2m  float64 `json:"relative_humidity_2m"`
		WindSpeed10m        float64 `json:"wind_speed_10m"`
		ApparentTemperature float64 `json:"apparent_temperature"`
		Time                string  `json:"time"`
	} `json:"current"`

	Daily struct {
		Time                 []string  `json:"time"`
		WeatherCode          []int     `json:"weather_code"`
		Temperature2mMin     []float64 `json:"temperature_2m_min"`
		Temperature2mMax     []float64 `json:"temperature_2m_max"`
		RelativeHumidityMean []float64 `json:"relative_humidity_2m_mean"`
	} `json:"daily"`
}

//
// ---------- VIEW MODELS ----------
//

type WeatherDay struct {
	Label     string
	DateShort string
	Icon      string
	Text      string
	TempMin   int
	TempMax   int
}

type WeatherView struct {
	City        string
	UpdatedAt   time.Time
	CurrentTemp int
	TodayMin    int
	TodayMax    int
	TodayText   string
	TodayIcon   string
	TodayTime   time.Time
	Days        []WeatherDay
	Humidity    int
	WindKmh     float64
	FeelsLike   int
}

//
// ---------- WEATHER CODE MAP ----------
//

type weatherDesc struct {
	Icon string // один символ: S,R,W,C,N,G
	Text string
}

var weatherMap = map[int]weatherDesc{
	0:  {Icon: "☀", Text: "Klar"},
	1:  {Icon: "🌤", Text: "Überwiegend klar"},
	2:  {Icon: "⛅", Text: "Teilw. bewölkt"},
	3:  {Icon: "☁", Text: "Bewölkt"},
	45: {Icon: "〰", Text: "Nebel"},
	48: {Icon: "〰", Text: "Nebel"},
	51: {Icon: "☂", Text: "Leichter Niesel"},
	53: {Icon: "☂", Text: "Nieselregen"},
	55: {Icon: "☂", Text: "Starker Niesel"},
	61: {Icon: "☂", Text: "Leichter Regen"},
	63: {Icon: "☂", Text: "Regen"},
	65: {Icon: "☂", Text: "Starker Regen"},
	71: {Icon: "❄", Text: "Leichter Schnee"},
	73: {Icon: "❄", Text: "Schnee"},
	75: {Icon: "❄", Text: "Starker Schnee"},
	80: {Icon: "☂", Text: "Schauer"},
	81: {Icon: "☂", Text: "Starke Schauer"},
	82: {Icon: "☂", Text: "Gewittrige Schauer"},
	95: {Icon: "⛈", Text: "Gewitter"},
	96: {Icon: "⛈", Text: "Gewitter & Hagel"},
	99: {Icon: "⛈", Text: "Starkes Gewitter"},
}

func w(code int) weatherDesc {
	if d, ok := weatherMap[code]; ok {
		return d
	}
	return weatherDesc{"·", "—"}
}

//
// ---------- THE FUNCTION YOU NEED ----------
//

func loadWeather(ctx context.Context) (*WeatherView, error) {
	// Нормальные координаты Дюссельдорфа
	const url = "https://api.open-meteo.com/v1/forecast" +
		"?latitude=51.2277&longitude=6.7735" +
		"&current=temperature_2m,weather_code,relative_humidity_2m,apparent_temperature,wind_speed_10m" +
		"&daily=weather_code,temperature_2m_max,temperature_2m_min,relative_humidity_2m_mean" +
		"&forecast_days=7" +
		"&timezone=Europe%2FBerlin"

	// Таймаут запроса (важно!)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// Короткий HTTP-клиент — НЕ зависнет
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weather fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status: %s", resp.Status)
	}

	var raw openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}

	// Timezone
	loc, _ := time.LoadLocation("Europe/Berlin")
	currentTime, _ := time.ParseInLocation(time.RFC3339, raw.CurrentWeather.Time, loc)

	// Подготовка view
	view := &WeatherView{
		City:        "Düsseldorf",
		UpdatedAt:   time.Now().In(loc),
		CurrentTemp: int(round(raw.CurrentWeather.Temperature2m)),
		TodayMin:    int(round(raw.Daily.Temperature2mMin[0])),
		TodayMax:    int(round(raw.Daily.Temperature2mMax[0])),
		Humidity:    int(raw.CurrentWeather.RelativeHumidity2m),
		TodayText:   w(raw.CurrentWeather.WeatherCode).Text,
		TodayIcon:   w(raw.CurrentWeather.WeatherCode).Icon,
		TodayTime:   currentTime,
		FeelsLike:   int(math.Round(raw.CurrentWeather.ApparentTemperature)),
		WindKmh:     raw.CurrentWeather.WindSpeed10m,
	}

	// Дни
	for i, dateStr := range raw.Daily.Time {
		date, _ := time.ParseInLocation("2006-01-02", dateStr, loc)

		label := func(i int) string {
			if i == 0 {
				return "Heute"
			}
			if i == 1 {
				return "Morgen"
			}
			return date.Format("Mon") // Mo / Di / Mi …
		}(i)

		wd := WeatherDay{
			Label:     label,
			DateShort: date.Format("02.01."),
			Icon:      w(raw.Daily.WeatherCode[i]).Icon,
			Text:      w(raw.Daily.WeatherCode[i]).Text,
			TempMin:   int(round(raw.Daily.Temperature2mMin[i])),
			TempMax:   int(round(raw.Daily.Temperature2mMax[i])),
		}
		view.Days = append(view.Days, wd)
	}

	return view, nil
}

func round(v float64) float64 {
	if v >= 0 {
		return float64(int(v + 0.5))
	}
	return float64(int(v - 0.5))
}

func renderWeatherBMP(ctx context.Context) ([]byte, error) {
	weatherView, err := loadWeather(ctx)
	if err != nil {
		log.Println("error fetching:", err)
		return nil, fmt.Errorf("unable to fetch weather: %w", err)
	}

	var buf bytes.Buffer
	if err := weatherTpl.Execute(&buf, weatherView); err != nil {
		log.Println("execute template:", err)
		return nil, fmt.Errorf("unable render quotes template: %w", err)
	}
	htmlStr := buf.String()

	png, err := htmlToBMP(htmlStr)
	if err != nil {
		log.Println("html to png:", err)
		return nil, fmt.Errorf("unable render bmp: %w", err)
	}

	return png, nil
}

func handleWeatherBMP(w http.ResponseWriter, r *http.Request) {
	serveCachedImage(w, r, &weatherCache)
}
