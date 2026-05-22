package image

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// EnsureFontDownloaded checks if the Inter-SemiBold font exists in the data directory.
// If it does not exist, it downloads it from Google Fonts repository.
func EnsureFontDownloaded() error {
	fontDir := "data"
	fontPath := filepath.Join(fontDir, "Inter-SemiBold.ttf")

	// Check if font already exists
	if _, err := os.Stat(fontPath); err == nil {
		return nil
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(fontDir, 0755); err != nil {
		return fmt.Errorf("failed to create font directory: %w", err)
	}

	// Download font from a reliable public repository via jsDelivr CDN
	url := "https://cdn.jsdelivr.net/gh/QuantConnect/Research@master/Explore/Inter%20font/static/Inter-SemiBold.ttf"
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to make request to download font: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download font, status: %s", resp.Status)
	}

	outFile, err := os.Create(fontPath)
	if err != nil {
		return fmt.Errorf("failed to create local font file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save font file: %w", err)
	}

	return nil
}

// GenerateCard creates a branded 1200x675 card image and returns its PNG bytes.
func GenerateCard(category string, title string, details string, source string) ([]byte, error) {
	// 1. Ensure the font is downloaded
	if err := EnsureFontDownloaded(); err != nil {
		return nil, fmt.Errorf("font setup failed: %w", err)
	}

	fontPath := filepath.Join("data", "Inter-SemiBold.ttf")
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read font file: %w", err)
	}

	parsedFont, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse font: %w", err)
	}

	// Create font faces at various sizes
	titleFace, err := opentype.NewFace(parsedFont, &opentype.FaceOptions{
		Size:    44,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create title font face: %w", err)
	}
	defer titleFace.Close()

	bodyFace, err := opentype.NewFace(parsedFont, &opentype.FaceOptions{
		Size:    26,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create body font face: %w", err)
	}
	defer bodyFace.Close()

	badgeFace, err := opentype.NewFace(parsedFont, &opentype.FaceOptions{
		Size:    18,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create badge font face: %w", err)
	}
	defer badgeFace.Close()

	// 2. Determine colors and styling based on category
	var (
		accentColor color.RGBA
		badgeColor  color.RGBA
		badgeText   string
	)

	// Clean category for checking
	normCat := strings.ToLower(strings.TrimSpace(category))
	switch normCat {
	case "emergency", "alert", "darurat":
		accentColor = color.RGBA{R: 239, G: 68, B: 68, A: 255} // Red 500
		badgeColor = color.RGBA{R: 185, G: 28, B: 28, A: 255}  // Red 700
		badgeText = "DARURAT"
	case "market", "jisdor", "kurs":
		accentColor = color.RGBA{R: 16, G: 185, B: 129, A: 255} // Emerald 500
		badgeColor = color.RGBA{R: 4, G: 120, B: 87, A: 255}   // Emerald 700
		badgeText = "KURS JISDOR"
	case "crypto", "bitcoin":
		accentColor = color.RGBA{R: 245, G: 158, B: 11, A: 255} // Amber 500
		badgeColor = color.RGBA{R: 180, G: 83, B: 9, A: 255}   // Amber 700
		badgeText = "CRYPTO UPDATE"
	case "news":
		accentColor = color.RGBA{R: 59, G: 130, B: 246, A: 255} // Blue 500
		badgeColor = color.RGBA{R: 29, G: 78, B: 216, A: 255}   // Blue 700
		badgeText = "NEWS UPDATE"
	default:
		accentColor = color.RGBA{R: 100, G: 116, B: 139, A: 255} // Slate 500
		badgeColor = color.RGBA{R: 71, G: 85, B: 105, A: 255}    // Slate 600
		badgeText = "INFO"
	}

	// 3. Create canvas (1200x675)
	rect := image.Rect(0, 0, 1200, 675)
	img := image.NewRGBA(rect)

	// Draw diagonal background gradient (slate 900 to slate 800)
	startBg := color.RGBA{R: 15, G: 23, B: 42, A: 255}  // #0f172a
	endBg := color.RGBA{R: 30, G: 41, B: 59, A: 255}    // #1e293b
	drawDiagonalGradient(img, rect, startBg, endBg)

	// Draw bottom accent strip (10px height)
	accentRect := image.Rect(0, 665, 1200, 675)
	drawRect(img, accentRect, accentColor)

	// 4. Render Badge (Top Left)
	badgeWidth := measureTextWidth(badgeFace, badgeText)
	badgeRect := image.Rect(80, 80, 80+badgeWidth+32, 80+36)
	drawRoundedRect(img, badgeRect, 6, badgeColor)
	drawText(img, badgeText, 80+16, 80+24, badgeFace, color.RGBA{255, 255, 255, 255})

	// Render Branding (Top Right)
	brandingText := "Before Tomorrow"
	brandWidth := measureTextWidth(badgeFace, brandingText)
	drawText(img, brandingText, 1200-80-brandWidth, 80+24, badgeFace, color.RGBA{148, 163, 184, 255}) // Slate 400

	// 5. Wrap and draw Title
	maxTextWidth := 1200 - 160 // 1040px width
	titleLines := wrapText(titleFace, title, maxTextWidth)

	titleStartY := 210
	titleLineHeight := 58
	for i, line := range titleLines {
		yPos := titleStartY + (i * titleLineHeight)
		// Limit to max 3 title lines to prevent overflow
		if i >= 3 {
			break
		}
		drawText(img, line, 80, yPos, titleFace, color.RGBA{255, 255, 255, 255})
	}

	// 6. Wrap and draw Body/Details
	bodyStartY := titleStartY + (len(titleLines) * titleLineHeight) + 10
	if len(titleLines) > 3 {
		bodyStartY = titleStartY + (3 * titleLineHeight) + 10
	}
	if bodyStartY < 340 {
		bodyStartY = 340 // Ensure minimum vertical spacing if title is short
	}

	bodyLines := wrapText(bodyFace, details, maxTextWidth)
	bodyLineHeight := 38
	for i, line := range bodyLines {
		yPos := bodyStartY + (i * bodyLineHeight)
		// Limit details so it doesn't overflow footer
		if yPos > 530 {
			break
		}
		drawText(img, line, 80, yPos, bodyFace, color.RGBA{203, 213, 225, 255}) // Slate 300
	}

	// 7. Draw Footer (Source details)
	footerY := 590
	footerText := "SUMBER: " + strings.ToUpper(strings.TrimSpace(source))
	if source == "" {
		footerText = "SUMBER: VALIDASI DIGITAL"
	}
	drawText(img, footerText, 80, footerY, badgeFace, color.RGBA{148, 163, 184, 255}) // Slate 400

	// 8. Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode png: %w", err)
	}

	return buf.Bytes(), nil
}

// Helpers

func drawDiagonalGradient(dst *image.RGBA, rect image.Rectangle, startColor, endColor color.RGBA) {
	dx := rect.Dx()
	dy := rect.Dy()
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			// t varies from 0 (top-left) to 1 (bottom-right)
			t := (float64(x-rect.Min.X)/float64(dx))*0.5 + (float64(y-rect.Min.Y)/float64(dy))*0.5
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			r := uint8(float64(startColor.R)*(1-t) + float64(endColor.R)*t)
			g := uint8(float64(startColor.G)*(1-t) + float64(endColor.G)*t)
			b := uint8(float64(startColor.B)*(1-t) + float64(endColor.B)*t)
			dst.SetRGBA(x, y, color.RGBA{r, g, b, 255})
		}
	}
}

func drawRect(dst *image.RGBA, rect image.Rectangle, c color.Color) {
	draw.Draw(dst, rect, image.NewUniform(c), image.Point{}, draw.Src)
}

func drawRoundedRect(dst *image.RGBA, rect image.Rectangle, r int, c color.RGBA) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			// Check if we are inside the corners
			if x < rect.Min.X+r && y < rect.Min.Y+r { // Top-left
				dx, dy := x-(rect.Min.X+r), y-(rect.Min.Y+r)
				if dx*dx+dy*dy > r*r {
					continue
				}
			} else if x >= rect.Max.X-r && y < rect.Min.Y+r { // Top-right
				dx, dy := x-(rect.Max.X-r), y-(rect.Min.Y+r)
				if dx*dx+dy*dy > r*r {
					continue
				}
			} else if x < rect.Min.X+r && y >= rect.Max.Y-r { // Bottom-left
				dx, dy := x-(rect.Min.X+r), y-(rect.Max.Y-r)
				if dx*dx+dy*dy > r*r {
					continue
				}
			} else if x >= rect.Max.X-r && y >= rect.Max.Y-r { // Bottom-right
				dx, dy := x-(rect.Max.X-r), y-(rect.Max.Y-r)
				if dx*dx+dy*dy > r*r {
					continue
				}
			}
			dst.SetRGBA(x, y, c)
		}
	}
}

func drawText(dst *image.RGBA, text string, x, y int, face font.Face, c color.Color) {
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(text)
}

func measureTextWidth(face font.Face, text string) int {
	w := font.MeasureString(face, text)
	return int(w >> 6)
}

func wrapText(face font.Face, text string, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	var currentLine string
	for _, word := range words {
		var testLine string
		if currentLine == "" {
			testLine = word
		} else {
			testLine = currentLine + " " + word
		}
		width := measureTextWidth(face, testLine)
		if width > maxWidth {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word
		} else {
			currentLine = testLine
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return lines
}
