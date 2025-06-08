// Copyright © 2020 The Homeport Team
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package img

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"strings"

	"github.com/esimov/stackblur-go"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/gonvenience/bunt"
	"github.com/gonvenience/font"
	"github.com/gonvenience/term"
	imgfont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

const (
	red    = "#ED655A"
	yellow = "#E1C04C"
	green  = "#71BD47"
)

const (
	defaultFontSize = 12
	defaultFontDPI  = 144
)

// commandIndicator is the string to be used to indicate the command in the screenshot
var commandIndicator = func() string {
	if val, ok := os.LookupEnv("TS_COMMAND_INDICATOR"); ok {
		return val
	}

	return "➜"
}()

type Scaffold struct {
	content bunt.String

	factor float64

	columns int

	defaultForegroundColor color.Color
	defaultBackgroundColor color.Color
	customColors           map[int]color.Color

	clipCanvas bool

	drawDecorations bool
	drawShadow      bool

	shadowBaseColor string
	shadowRadius    uint8
	shadowOffsetX   float64
	shadowOffsetY   float64

	padding float64
	margin  float64

	regular     imgfont.Face
	bold        imgfont.Face
	italic      imgfont.Face
	boldItalic  imgfont.Face
	lineSpacing float64
	tabSpaces   int
}

func NewImageCreator() Scaffold {
	f := 2.0

	fontFaceOptions := &truetype.Options{
		Size: f * defaultFontSize,
		DPI:  defaultFontDPI,
	}

	return Scaffold{
		defaultForegroundColor: bunt.LightGray,
		defaultBackgroundColor: color.RGBA{R: 0x15, G: 0x15, B: 0x15, A: 255}, // #151515

		factor: f,

		margin:  f * 48,
		padding: f * 24,

		drawDecorations: true,
		drawShadow:      true,

		shadowBaseColor: "#10101066",
		shadowRadius:    uint8(math.Min(f*16, 255)),
		shadowOffsetX:   f * 16,
		shadowOffsetY:   f * 16,

		regular:    font.Hack.Regular(fontFaceOptions),
		bold:       font.Hack.Bold(fontFaceOptions),
		italic:     font.Hack.Italic(fontFaceOptions),
		boldItalic: font.Hack.BoldItalic(fontFaceOptions),

		lineSpacing: 1.2,
		tabSpaces:   2,
	}
}

func (s *Scaffold) SetFontFaceRegular(face imgfont.Face) { s.regular = face }

func (s *Scaffold) SetFontFaceBold(face imgfont.Face) { s.bold = face }

func (s *Scaffold) SetFontFaceItalic(face imgfont.Face) { s.italic = face }

func (s *Scaffold) SetFontFaceBoldItalic(face imgfont.Face) { s.boldItalic = face }

func (s *Scaffold) SetColumns(columns int) { s.columns = columns }

func (s *Scaffold) DrawDecorations(value bool) { s.drawDecorations = value }

func (s *Scaffold) DrawShadow(value bool) { s.drawShadow = value }

func (s *Scaffold) ClipCanvas(value bool) { s.clipCanvas = value }

// LoadCustomFonts loads custom fonts from file paths, applying them in order
func (s *Scaffold) LoadCustomFonts(fontPaths []string) error {
	fontFaceOptions := &truetype.Options{
		Size: s.factor * defaultFontSize,
		DPI:  defaultFontDPI,
	}

	for i, fontPath := range fontPaths {
		fontBytes, err := os.ReadFile(fontPath)
		if err != nil {
			return fmt.Errorf("failed to read font file %s: %w", fontPath, err)
		}

		var face imgfont.Face
		if strings.HasSuffix(strings.ToLower(fontPath), ".ttf") {
			ttfFont, err := truetype.Parse(fontBytes)
			if err != nil {
				return fmt.Errorf("failed to parse TTF font %s: %w", fontPath, err)
			}
			face = truetype.NewFace(ttfFont, fontFaceOptions)
		} else {
			otfFont, err := opentype.Parse(fontBytes)
			if err != nil {
				return fmt.Errorf("failed to parse font %s: %w", fontPath, err)
			}
			face, err = opentype.NewFace(otfFont, &opentype.FaceOptions{
				Size: s.factor * defaultFontSize,
				DPI:  defaultFontDPI,
			})
			if err != nil {
				return fmt.Errorf("failed to create font face for %s: %w", fontPath, err)
			}
		}

		// Apply fonts in order: regular, bold, italic, boldItalic
		// If only one font is provided, use it for all variants
		switch i % 4 {
		case 0:
			s.regular = face
			// If only one font provided, use it for all variants
			if len(fontPaths) == 1 {
				s.bold = face
				s.italic = face
				s.boldItalic = face
			}
		case 1:
			s.bold = face
		case 2:
			s.italic = face
		case 3:
			s.boldItalic = face
		}
	}

	return nil
}

// LoadColorscheme loads a custom colorscheme from a JSON file
func (s *Scaffold) LoadColorscheme(colorschemeFile string) error {
	data, err := os.ReadFile(colorschemeFile)
	if err != nil {
		return fmt.Errorf("failed to read colorscheme file: %w", err)
	}

	s.customColors = make(map[int]color.Color)

	// Try parsing as array first (your format)
	var schemeArray []struct {
		Colors map[string]string `json:"colors"`
	}
	
	if err := json.Unmarshal(data, &schemeArray); err == nil && len(schemeArray) > 0 {
		// Use first scheme in array
		scheme := schemeArray[0]
		for i := 0; i < 16; i++ {
			colorKey := fmt.Sprintf("color%d", i)
			if hexColor, exists := scheme.Colors[colorKey]; exists {
				c, err := parseHexColor(hexColor)
				if err != nil {
					return fmt.Errorf("invalid color %s for %s: %w", hexColor, colorKey, err)
				}
				s.customColors[i] = c
			}
		}
		
		// Apply custom foreground color if specified
		if foregroundHex, exists := scheme.Colors["foreground"]; exists {
			c, err := parseHexColor(foregroundHex)
			if err != nil {
				return fmt.Errorf("invalid foreground color %s: %w", foregroundHex, err)
			}
			s.defaultForegroundColor = c
		}
		
		// Apply custom background color if specified
		if backgroundHex, exists := scheme.Colors["background"]; exists {
			c, err := parseHexColor(backgroundHex)
			if err != nil {
				return fmt.Errorf("invalid background color %s: %w", backgroundHex, err)
			}
			s.defaultBackgroundColor = c
		}
		
		return nil
	}

	// Try parsing as single object (simple format)
	var scheme struct {
		Colors map[string]string `json:"colors"`
	}

	if err := json.Unmarshal(data, &scheme); err != nil {
		return fmt.Errorf("failed to parse colorscheme JSON: %w", err)
	}

	for i := 0; i < 16; i++ {
		colorKey := fmt.Sprintf("color%d", i)
		if hexColor, exists := scheme.Colors[colorKey]; exists {
			c, err := parseHexColor(hexColor)
			if err != nil {
				return fmt.Errorf("invalid color %s for %s: %w", hexColor, colorKey, err)
			}
			s.customColors[i] = c
		}
	}
	
	// Apply custom foreground color if specified
	if foregroundHex, exists := scheme.Colors["foreground"]; exists {
		c, err := parseHexColor(foregroundHex)
		if err != nil {
			return fmt.Errorf("invalid foreground color %s: %w", foregroundHex, err)
		}
		s.defaultForegroundColor = c
	}
	
	// Apply custom background color if specified
	if backgroundHex, exists := scheme.Colors["background"]; exists {
		c, err := parseHexColor(backgroundHex)
		if err != nil {
			return fmt.Errorf("invalid background color %s: %w", backgroundHex, err)
		}
		s.defaultBackgroundColor = c
	}

	return nil
}

// parseHexColor converts a hex color string to color.Color
func parseHexColor(hexStr string) (color.Color, error) {
	hexStr = strings.TrimPrefix(hexStr, "#")
	if len(hexStr) != 6 {
		return nil, fmt.Errorf("hex color must be 6 characters long")
	}

	rgb, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex color: %w", err)
	}

	return color.RGBA{R: rgb[0], G: rgb[1], B: rgb[2], A: 255}, nil
}

// getColor returns the appropriate color based on ANSI color index and custom colorscheme
func (s *Scaffold) getColor(ansiColorIndex int, fallbackColor color.Color) color.Color {
	if s.customColors != nil {
		if customColor, exists := s.customColors[ansiColorIndex]; exists {
			return customColor
		}
	}
	return fallbackColor
}

// mapStandardColor attempts to map standard ANSI RGB values to custom colors
func (s *Scaffold) mapStandardColor(r, g, b int) (color.Color, bool) {
	if s.customColors == nil {
		return nil, false
	}

	// Common standard ANSI color mappings (these are typical values)
	standardColors := map[[3]int]int{
		{0, 0, 0}:       0,  // black
		{128, 0, 0}:     1,  // red
		{0, 128, 0}:     2,  // green
		{128, 128, 0}:   3,  // yellow
		{0, 0, 128}:     4,  // blue
		{128, 0, 128}:   5,  // magenta
		{0, 128, 128}:   6,  // cyan
		{192, 192, 192}: 7,  // light gray
		{128, 128, 128}: 8,  // dark gray
		{255, 0, 0}:     9,  // light red
		{0, 255, 0}:     10, // light green
		{255, 255, 0}:   11, // light yellow
		{0, 0, 255}:     12, // light blue
		{255, 0, 255}:   13, // light magenta
		{0, 255, 255}:   14, // light cyan
		{255, 255, 255}: 15, // white
	}

	if colorIndex, found := standardColors[[3]int{r, g, b}]; found {
		if customColor, exists := s.customColors[colorIndex]; exists {
			return customColor, true
		}
	}

	return nil, false
}

func (s *Scaffold) GetFixedColumns() int {
	if s.columns != 0 {
		return s.columns
	}

	columns, _ := term.GetTerminalSize()
	return columns
}

func (s *Scaffold) AddCommand(args ...string) error {
	return s.AddContent(strings.NewReader(
		bunt.Sprintf("Lime{%s} DimGray{%s}\n",
			commandIndicator,
			strings.Join(args, " "),
		),
	))
}

func (s *Scaffold) AddContent(in io.Reader) error {
	parsed, err := bunt.ParseStream(in)
	if err != nil {
		return fmt.Errorf("failed to parse input stream: %w", err)
	}

	var tmp bunt.String
	var counter int
	for _, cr := range *parsed {
		counter++

		if cr.Symbol == '\n' {
			counter = 0
		}

		// Add an additional newline in case the column
		// count is reached and line wrapping is needed
		if counter > s.GetFixedColumns() {
			counter = 0
			tmp = append(tmp, bunt.ColoredRune{
				Settings: cr.Settings,
				Symbol:   '\n',
			})
		}

		tmp = append(tmp, cr)
	}

	s.content = append(s.content, tmp...)

	return nil
}

func (s *Scaffold) fontHeight() float64 {
	return float64(s.regular.Metrics().Height >> 6)
}

func (s *Scaffold) measureContent() (width float64, height float64) {
	var tmp = make([]rune, len(s.content))
	for i, cr := range s.content {
		tmp[i] = cr.Symbol
	}

	lines := strings.Split(
		strings.TrimSuffix(
			string(tmp),
			"\n",
		),
		"\n",
	)

	// temporary drawer for reference calucation
	tmpDrawer := &imgfont.Drawer{Face: s.regular}

	// width, either by using longest line, or by fixed column value
	switch s.columns {
	case 0: // unlimited: max width of all lines
		for _, line := range lines {
			advance := tmpDrawer.MeasureString(line)
			if lineWidth := float64(advance >> 6); lineWidth > width {
				width = lineWidth
			}
		}

	default: // fixed: max width based on column count
		width = float64(tmpDrawer.MeasureString(strings.Repeat("a", s.GetFixedColumns())) >> 6)
	}

	// height, lines times font height and line spacing
	height = float64(len(lines)) * s.fontHeight() * s.lineSpacing

	return width, height
}

func (s *Scaffold) image() (image.Image, error) {
	var f = func(value float64) float64 { return s.factor * value }

	var (
		corner   = f(6)
		radius   = f(9)
		distance = f(25)
	)

	contentWidth, contentHeight := s.measureContent()

	// Make sure the output window is big enough in case no content or very few
	// content will be rendered
	contentWidth = math.Max(contentWidth, 3*distance+3*radius)

	marginX, marginY := s.margin, s.margin
	paddingX, paddingY := s.padding, s.padding

	xOffset := marginX
	yOffset := marginY

	var titleOffset float64
	if s.drawDecorations {
		titleOffset = f(40)
	}

	width := contentWidth + 2*marginX + 2*paddingX
	height := contentHeight + 2*marginY + 2*paddingY + titleOffset

	dc := gg.NewContext(int(width), int(height))

	// Optional: Apply blurred rounded rectangle to mimic the window shadow
	//
	if s.drawShadow {
		xOffset -= s.shadowOffsetX / 2
		yOffset -= s.shadowOffsetY / 2

		bc := gg.NewContext(int(width), int(height))
		bc.DrawRoundedRectangle(xOffset+s.shadowOffsetX, yOffset+s.shadowOffsetY, width-2*marginX, height-2*marginY, corner)
		bc.SetHexColor(s.shadowBaseColor)
		bc.Fill()

		shadow, err := stackblur.Process(bc.Image(), uint32(s.shadowRadius))
		if err != nil {
			return nil, err
		}

		dc.DrawImage(shadow, 0, 0)
	}

	// Draw rounded rectangle with outline to produce impression of a window
	//
	dc.DrawRoundedRectangle(xOffset, yOffset, width-2*marginX, height-2*marginY, corner)
	dc.SetColor(s.defaultBackgroundColor)
	dc.Fill()

	dc.DrawRoundedRectangle(xOffset, yOffset, width-2*marginX, height-2*marginY, corner)
	dc.SetHexColor("#404040")
	dc.SetLineWidth(f(1))
	dc.Stroke()

	// Optional: Draw window decorations (i.e. three buttons) to produce the
	// impression of an actional window
	//
	if s.drawDecorations {
		for i, color := range []string{red, yellow, green} {
			dc.DrawCircle(xOffset+paddingX+float64(i)*distance+f(4), yOffset+paddingY+f(4), radius)
			dc.SetHexColor(color)
			dc.Fill()
		}
	}

	// Apply the actual text into the prepared content area of the window
	//
	var x, y = xOffset + paddingX, yOffset + paddingY + titleOffset + s.fontHeight()
	for _, cr := range s.content {
		switch cr.Settings & 0x1C {
		case 4:
			dc.SetFontFace(s.bold)

		case 8:
			dc.SetFontFace(s.italic)

		case 12:
			dc.SetFontFace(s.boldItalic)

		default:
			dc.SetFontFace(s.regular)
		}

		str := string(cr.Symbol)
		w, h := dc.MeasureString(str)

		// background color
		switch cr.Settings & 0x02 { //nolint:gocritic
		case 2:
			r := int((cr.Settings>>32)&0xFF) // #nosec G115
			g := int((cr.Settings>>40)&0xFF) // #nosec G115
			b := int((cr.Settings>>48)&0xFF) // #nosec G115
			
			if customColor, found := s.mapStandardColor(r, g, b); found {
				dc.SetColor(customColor)
			} else {
				dc.SetRGB255(r, g, b)
			}

			dc.DrawRectangle(x, y-h+12, w, h)
			dc.Fill()
		}

		// foreground color
		switch cr.Settings & 0x01 {
		case 1:
			r := int((cr.Settings>>8)&0xFF)  // #nosec G115
			g := int((cr.Settings>>16)&0xFF) // #nosec G115
			b := int((cr.Settings>>24)&0xFF) // #nosec G115
			
			if customColor, found := s.mapStandardColor(r, g, b); found {
				dc.SetColor(customColor)
			} else {
				dc.SetRGB255(r, g, b)
			}

		default:
			dc.SetColor(s.defaultForegroundColor)
		}

		switch str {
		case "\n":
			x = xOffset + paddingX
			y += h * s.lineSpacing
			continue

		case "\t":
			x += w * float64(s.tabSpaces)
			continue

		case "✗", "ˣ": // mitigate issue #1 by replacing it with a similar character
			str = "×"
		}

		dc.DrawString(str, x, y)

		// There seems to be no font face based way to do an underlined
		// string, therefore manually draw a line under each character
		if cr.Settings&0x1C == 16 {
			dc.DrawLine(x, y+f(4), x+w, y+f(4))
			dc.SetLineWidth(f(1))
			dc.Stroke()
		}

		x += w
	}

	return dc.Image(), nil
}

// Write writes the scaffold content as PNG into the provided writer
//
// Deprecated: Use [Scaffold.WritePNG] instead.
func (s *Scaffold) Write(w io.Writer) error {
	return s.WritePNG(w)
}

// WritePNG writes the scaffold content as PNG into the provided writer
func (s *Scaffold) WritePNG(w io.Writer) error {
	img, err := s.image()
	if err != nil {
		return err
	}

	// Optional: Clip image to minimum size by removing all surrounding transparent pixels
	//
	if s.clipCanvas {
		if imgRGBA, ok := img.(*image.RGBA); ok {
			var minX, minY = math.MaxInt, math.MaxInt
			var maxX, maxY = 0, 0

			var bounds = imgRGBA.Bounds()
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
					r, g, b, a := imgRGBA.At(x, y).RGBA()
					isTransparent := r == 0 && g == 0 && b == 0 && a == 0

					if !isTransparent {
						if x < minX {
							minX = x
						}

						if y < minY {
							minY = y
						}

						if x > maxX {
							maxX = x
						}

						if y > maxY {
							maxY = y
						}
					}
				}
			}

			img = imgRGBA.SubImage(image.Rect(minX, minY, maxX, maxY))
		}
	}

	return png.Encode(w, img)
}

// WriteRaw writes the scaffold content as-is into the provided writer
func (s *Scaffold) WriteRaw(w io.Writer) error {
	_, err := w.Write([]byte(s.content.String()))
	return err
}
