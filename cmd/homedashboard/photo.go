package main

import (
	"bytes"
	"context"
	_ "embed"
	"image"
	_ "image/jpeg"
	"net/http"

	"golang.org/x/image/bmp"
	"golang.org/x/image/draw"
)

//go:embed assets/photo.jpg
var photoBytes []byte

func renderPhotoBMP(ctx context.Context) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(photoBytes))
	if err != nil {
		return nil, err
	}
	_ = format // можно логировать

	// ресайз до 800x480
	// cover-масштабирование в 800x480 без искажений
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))

	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	targetW, targetH := dst.Bounds().Dx(), dst.Bounds().Dy()

	targetRatio := float64(targetW) / float64(targetH)
	srcRatio := float64(srcW) / float64(srcH)

	var srcRect image.Rectangle

	if srcRatio > targetRatio {
		// картинка шире, чем надо – режем по бокам
		newW := int(float64(srcH) * targetRatio)
		offsetX := (srcW - newW) / 2

		srcRect = image.Rect(
			bounds.Min.X+offsetX,
			bounds.Min.Y,
			bounds.Min.X+offsetX+newW,
			bounds.Min.Y+srcH,
		)
	} else {
		// картинка выше/уже – режем сверху/снизу
		newH := int(float64(srcW) / targetRatio)
		offsetY := (srcH - newH) / 2

		srcRect = image.Rect(
			bounds.Min.X,
			bounds.Min.Y+offsetY,
			bounds.Min.X+srcW,
			bounds.Min.Y+offsetY+newH,
		)
	}

	// растягиваем выбранный кусок на весь экран 800x480
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, srcRect, draw.Over, nil)

	// твой дезеринг
	dithered := ditherFloydSteinbergHybrid(dst, true)

	// кодируем в BMP в память
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, dithered); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func handlePhotoBMP(w http.ResponseWriter, r *http.Request) {
	serveCachedImage(w, r, &photoCache)
}
