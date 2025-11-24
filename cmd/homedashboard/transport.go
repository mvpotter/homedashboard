package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
)

func renderTransportBMP(ctx context.Context) ([]byte, error) {
	data := ""
	var buf bytes.Buffer
	if err := transportTpl.Execute(&buf, data); err != nil {
		log.Println("execute template:", err)
		return nil, fmt.Errorf("unable render transport template: %w", err)
	}
	htmlStr := buf.String()

	png, err := htmlToBMP(htmlStr)
	if err != nil {
		log.Println("html to bmp:", err)
		return nil, fmt.Errorf("unable render bmp: %w", err)
	}

	return png, nil
}

func handleTransportBMP(w http.ResponseWriter, r *http.Request) {
	serveCachedImage(w, r, &transportCache)
}
