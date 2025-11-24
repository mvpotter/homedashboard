package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func renderQuoteBMP(ctx context.Context) ([]byte, error) {
	resp, err := http.Get("https://api.zitat-service.de/v1/quote?language=de")
	if err != nil {
		log.Println("error fetching:", err)
		return nil, fmt.Errorf("unable to fetch quotes: %w", err)
	}
	defer resp.Body.Close()

	var data map[string]any
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		log.Println("decode error:", err)
		return nil, fmt.Errorf("unable to decode quotes json: %w", err)
	}

	var buf bytes.Buffer
	if err := quoteTpl.Execute(&buf, data); err != nil {
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

func handleQuoteBMP(w http.ResponseWriter, r *http.Request) {
	serveCachedImage(w, r, &quoteCache)
}
