package main

import (
	"context"
	"log"
	"time"
)

func startBackgroundRenderer(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				loadAllImages(ctx)
			case <-ctx.Done():
				log.Println("background renderer stopped:", ctx.Err())
				return
			}
		}
	}()
}

func loadAllImages(ctx context.Context) {
	// Transport
	if bmp, err := renderTransportBMP(ctx); err == nil {
		transportCache.Set(bmp)
	} else {
		log.Println("renderCalendarBMP error:", err)
	}

	// Weather
	if bmp, err := renderWeatherBMP(ctx); err == nil {
		weatherCache.Set(bmp)
	} else {
		log.Println("renderWeatherBMP error:", err)
	}

	// Quote
	if bmp, err := renderQuoteBMP(ctx); err == nil {
		quoteCache.Set(bmp)
	} else {
		log.Println("renderQuoteBMP error:", err)
	}

	// Photo
	if len(photoCache.data) == 0 {
		if bmp, err := renderPhotoBMP(ctx); err == nil {
			photoCache.Set(bmp)
		} else {
			log.Println("renderPhotoBMP error:", err)
		}
	}

	// Stocks
	if bmp, err := renderVWCEBMP(ctx); err == nil {
		stocksCache.Set(bmp)
	} else {
		log.Println("renderStocksBMP error:", err)
	}
}
