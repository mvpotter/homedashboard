package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
)

// стандартная матрица Bayer 8x8 (значения 0..63)
var bayer8x8 = [8][8]uint8{
	{0, 32, 8, 40, 2, 34, 10, 42},
	{48, 16, 56, 24, 50, 18, 58, 26},
	{12, 44, 4, 36, 14, 46, 6, 38},
	{60, 28, 52, 20, 62, 30, 54, 22},
	{3, 35, 11, 43, 1, 33, 9, 41},
	{51, 19, 59, 27, 49, 17, 57, 25},
	{15, 47, 7, 39, 13, 45, 5, 37},
	{63, 31, 55, 23, 61, 29, 53, 21},
}

// Bayer 8x8 dithering -> ч/б 24-битное изображение
// invert = true если хочешь инвертировать (чёрное <-> белое)
func ditherBayer8x8(src image.Image, invert bool) *image.RGBA {
	bounds := src.Bounds()
	out := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		by := y & 7 // y % 8, но быстрее
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			bx := x & 7 // x % 8

			r, g, b, _ := src.At(x, y).RGBA()
			// яркость 0..65535 (примерно, без деления на 1000)
			lum := 299*r + 587*g + 114*b

			// нормализуем до 0..63 (сдвиг на 10 бит ~ деление на 1024)
			level := uint8((lum >> 10) & 0x3F)

			// threshold из Bayer-матрицы
			thr := bayer8x8[by][bx]

			// "тёмный" => чёрный пиксель
			isBlack := level < thr

			if invert {
				isBlack = !isBlack
			}

			if isBlack {
				out.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			} else {
				out.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}

	return out
}

// Матрица Bayer 4x4 (0..15)
var bayer4x4 = [4][4]uint8{
	{0, 8, 2, 10},
	{12, 4, 14, 6},
	{3, 11, 1, 9},
	{15, 7, 13, 5},
}

// Ordered dither Bayer 4x4 -> ч/б 24-бит
// invert = true, если нужно инвертировать чёрное/белое
func ditherBayer4x4(src image.Image, invert bool) *image.RGBA {
	bounds := src.Bounds()
	out := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		by := y & 3 // y % 4
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			bx := x & 3 // x % 4

			r, g, b, _ := src.At(x, y).RGBA()
			// яркость 0..65535
			lum := 299*r + 587*g + 114*b

			// нормализуем до 0..15 (65535 / 16 ≈ 4096 → сдвиг на 12 бит)
			level := uint8((lum >> 12) & 0x0F)

			thr := bayer4x4[by][bx]

			isBlack := level < thr
			if invert {
				isBlack = !isBlack
			}

			if isBlack {
				out.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			} else {
				out.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}

	return out
}

func ditherBayer4x4Hybrid(src image.Image, invert bool) *image.RGBA {
	bounds := src.Bounds()
	out := image.NewRGBA(bounds)

	// яркость у нас в диапазоне 0..~65535 (после формулы)
	const lowThreshold = 18000 // можно поиграть
	const highThreshold = 52000

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		by := y & 3
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			bx := x & 3

			r, g, b, _ := src.At(x, y).RGBA()
			lum := 299*r + 587*g + 114*b

			var isBlack bool

			switch {
			case lum < lowThreshold:
				// однозначно тёмный -> чёрный без dithering
				isBlack = true
			case lum > highThreshold:
				// однозначно светлый -> белый без dithering
				isBlack = false
			default:
				// средняя зона -> применяем Bayer 4x4
				level := uint8((lum >> 12) & 0x0F) // 0..15
				thr := bayer4x4[by][bx]
				isBlack = level < thr
			}

			if invert {
				isBlack = !isBlack
			}

			if isBlack {
				out.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			} else {
				out.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}

	return out
}

func encode1bppBMP(img image.Image) ([]byte, error) {
	b := img.Bounds()
	width := b.Dx()
	height := b.Dy()

	// bytes на строку без паддинга
	rawRowBytes := (width + 7) / 8
	// stride с выравниванием до 4 байт (BMP-требование)
	rowSize := (rawRowBytes + 3) &^ 3
	imageSize := rowSize * height

	// 14 (file) + 40 (DIB) + 8 (2 цвета палитры)
	const fileHeaderSize = 14
	const dibHeaderSize = 40
	const paletteSize = 8
	pixelOffset := fileHeaderSize + dibHeaderSize + paletteSize
	fileSize := uint32(pixelOffset + imageSize)

	buf := &bytes.Buffer{}
	buf.Grow(int(fileSize))

	// --- FILE HEADER (14 байт) ---
	// Signature "BM"
	buf.Write([]byte{'B', 'M'})

	// File size
	binary.Write(buf, binary.LittleEndian, fileSize)

	// Reserved1, Reserved2
	binary.Write(buf, binary.LittleEndian, uint16(0))
	binary.Write(buf, binary.LittleEndian, uint16(0))

	// Pixel data offset
	binary.Write(buf, binary.LittleEndian, uint32(pixelOffset))

	// --- DIB HEADER (BITMAPINFOHEADER, 40 байт) ---
	binary.Write(buf, binary.LittleEndian, uint32(dibHeaderSize)) // header size
	binary.Write(buf, binary.LittleEndian, int32(width))          // width
	binary.Write(buf, binary.LittleEndian, int32(height))         // height (bottom-up)
	binary.Write(buf, binary.LittleEndian, uint16(1))             // planes
	binary.Write(buf, binary.LittleEndian, uint16(1))             // bit count = 1
	binary.Write(buf, binary.LittleEndian, uint32(0))             // compression = BI_RGB
	binary.Write(buf, binary.LittleEndian, uint32(imageSize))     // image size
	binary.Write(buf, binary.LittleEndian, int32(0))              // x pixels per meter
	binary.Write(buf, binary.LittleEndian, int32(0))              // y pixels per meter
	binary.Write(buf, binary.LittleEndian, uint32(2))             // colors used
	binary.Write(buf, binary.LittleEndian, uint32(2))             // important colors

	// --- PALETTE (2 цвета, BGRA) ---
	// index 0 = white
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0x00}) // B,G,R,A
	// index 1 = black
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00})

	// --- PIXELS (bottom-up, 1 бит на пиксель, MSB first) ---
	// идём снизу вверх
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		var currentByte uint8
		bitPos := 7
		writtenInRow := 0

		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, b2, _ := img.At(x, y).RGBA()
			// считаем пиксель чёрным, если яркость меньше порога
			lum := 299*r + 587*g + 114*b2
			isBlack := lum < 32768

			if isBlack {
				// 1 = black (index 1 в палитре)
				currentByte |= 1 << uint(bitPos)
			}
			bitPos--
			if bitPos < 0 {
				buf.WriteByte(currentByte)
				writtenInRow++
				currentByte = 0
				bitPos = 7
			}
		}

		// если строка не на байтовой границе — дописываем хвост
		if bitPos != 7 {
			buf.WriteByte(currentByte)
			writtenInRow++
		}

		// добиваем паддинг до rowSize
		for writtenInRow < rowSize {
			buf.WriteByte(0x00)
			writtenInRow++
		}
	}

	return buf.Bytes(), nil
}

func ditherBayer8x8Hybrid(src image.Image, invert bool) *image.RGBA {
	bounds := src.Bounds()
	out := image.NewRGBA(bounds)

	const lowThreshold = 18000
	const highThreshold = 52000

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		by := y & 7 // 0..7
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			bx := x & 7 // 0..7

			r, g, b, _ := src.At(x, y).RGBA()
			// предполагаю, что у тебя где-то выше было >> 16,
			// чтобы получить 0..~65535; если нет — верни.
			lum := 299*r + 587*g + 114*b
			lum >>= 16 // теперь 0..~65535

			var isBlack bool
			switch {
			case lum < lowThreshold:
				isBlack = true
			case lum > highThreshold:
				isBlack = false
			default:
				// 0..63
				level := uint8((lum >> 10) & 0x3F)
				thr := bayer8x8[by][bx]
				isBlack = level < thr
			}

			if invert {
				isBlack = !isBlack
			}

			if isBlack {
				out.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			} else {
				out.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}

	return out
}

func ditherFloydSteinberg(src image.Image, invert bool) *image.RGBA {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	out := image.NewRGBA(bounds)

	// буфер яркости 0..1
	buf := make([]float64, w*h)

	// инициализируем из src
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			// r,g,b: 0..65535
			gray := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 65535.0
			buf[y*w+x] = gray
		}
	}

	// сам алгоритм
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			old := buf[i]
			var new float64
			if old >= 0.5 {
				new = 1.0
			} else {
				new = 0.0
			}
			err := old - new

			// ставим пиксель
			isBlack := (new < 0.5)
			if invert {
				isBlack = !isBlack
			}
			if isBlack {
				out.SetRGBA(bounds.Min.X+x, bounds.Min.Y+y, color.RGBA{0, 0, 0, 255})
			} else {
				out.SetRGBA(bounds.Min.X+x, bounds.Min.Y+y, color.RGBA{255, 255, 255, 255})
			}

			// разбрасываем ошибку (классическая маска Floyd–Steinberg)
			spread := func(xx, yy int, factor float64) {
				if xx < 0 || xx >= w || yy < 0 || yy >= h {
					return
				}
				buf[yy*w+xx] += err * factor
			}

			spread(x+1, y, 7.0/16.0)
			spread(x-1, y+1, 3.0/16.0)
			spread(x, y+1, 5.0/16.0)
			spread(x+1, y+1, 1.0/16.0)
		}
	}

	return out
}
