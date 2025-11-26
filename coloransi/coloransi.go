package coloransi

import (
	"fmt"
	"math/rand"
	"strings"
)

// ColorCode represents ANSI color codes and RGB colors as a 32-bit integer.
// The lower 8 bits represent ANSI color codes, and the upper 24 bits represent RGB values.
type ColorCode uint32

func (c ColorCode) CalculateLuminance() float64 {
	r, g, b := c.GetRGB()

	// Using relative luminance formula (perceived brightness)
	// See: https://www.w3.org/TR/WCAG20/#relativeluminancedef
	rr := float64(r) / 255.0
	gg := float64(g) / 255.0
	bb := float64(b) / 255.0

	return 0.2126*rr + 0.7152*gg + 0.0722*bb
}

// Appoximate RGB values for ANSI colors
func (c ColorCode) GetRGB() (uint8, uint8, uint8) {
	if c.IsRGB() {
		return uint8((c >> 24) & 0xFF),
			uint8((c >> 16) & 0xFF),
			uint8((c >> 8) & 0xFF)
	}

	// Convert ANSI colors to approximate RGB values
	// note these are approximate because a user can configure their terminal colors
	switch c {
	case Black:
		return 0, 0, 0
	case Red:
		return 170, 0, 0
	case Green:
		return 0, 170, 0
	case Yellow:
		return 170, 170, 0
	case Blue:
		return 0, 0, 170
	case Magenta:
		return 170, 0, 170
	case Cyan:
		return 0, 170, 170
	case White:
		return 170, 170, 170
	case BrightBlack:
		return 85, 85, 85
	case BrightRed:
		return 255, 85, 85
	case BrightGreen:
		return 85, 255, 85
	case BrightYellow:
		return 255, 255, 85
	case BrightBlue:
		return 85, 85, 255
	case BrightMagenta:
		return 255, 85, 255
	case BrightCyan:
		return 85, 255, 255
	case BrightWhite:
		return 255, 255, 255
	default:
		return 170, 170, 170 // Default to gray if unknown
	}
}

func (c ColorCode) GetContrast() ColorCode {
	luminance := c.CalculateLuminance()
	if luminance > 0.5 {
		return Black
	}
	return ColorWhite
}

// ANSI color codes
const (
	Black   ColorCode = 30
	Red     ColorCode = 31
	Green   ColorCode = 32
	Yellow  ColorCode = 33
	Blue    ColorCode = 34
	Magenta ColorCode = 35
	Cyan    ColorCode = 36
	White   ColorCode = 37

	// For bright colors, add 60
	BrightBlack   ColorCode = Black + 60
	BrightRed     ColorCode = Red + 60
	BrightGreen   ColorCode = Green + 60
	BrightYellow  ColorCode = Yellow + 60
	BrightBlue    ColorCode = Blue + 60
	BrightMagenta ColorCode = Magenta + 60
	BrightCyan    ColorCode = Cyan + 60
	BrightWhite   ColorCode = White + 60

	// Background colors start at 40, bright background colors at 100
	BackgroundOffset       ColorCode = 10
	BrightBackgroundOffset ColorCode = 60

	// RGB color mask
	RGBMask ColorCode = 0xFFFFFF00
)

type TextStyle uint8

const (
	Bold      TextStyle = 1
	Dim       TextStyle = 2
	Italic    TextStyle = 3
	Underline TextStyle = 4
	Blink     TextStyle = 5
	FastBlink TextStyle = 6
	Reverse   TextStyle = 7
	Hidden    TextStyle = 8
	Strike    TextStyle = 9
)

// Additional static RGB color definitions
func CreateRGB(r, g, b uint8) ColorCode {
	return ColorCode(uint32(r)<<24 | uint32(g)<<16 | uint32(b)<<8)
}

// Additional static RGB color definitions
var ColorOrange ColorCode = CreateRGB(255, 140, 0)
var ColorPink ColorCode = CreateRGB(255, 192, 203)
var ColorPurple ColorCode = CreateRGB(128, 0, 128)
var ColorTeal ColorCode = CreateRGB(0, 128, 128)
var ColorLimeGreen ColorCode = CreateRGB(50, 205, 50)
var ColorIndigo ColorCode = CreateRGB(75, 0, 130)
var ColorWhite ColorCode = CreateRGB(255, 255, 255)

// RGB creates a ColorCode from RGB values
func RGB(r, g, b uint8) ColorCode {
	return ColorCode(uint32(r)<<24 | uint32(g)<<16 | uint32(b)<<8)
}

// IsRGB checks if the ColorCode represents an RGB color
func (c ColorCode) IsRGB() bool {
	return c&RGBMask != 0
}

// Style formats the text with the specified text style, legacy
func Style(style TextStyle, v ...interface{}) string {
	return Styles([]TextStyle{style}, v...)
}

// Styles formats the text with the specified text styles
func Styles(styles []TextStyle, v ...interface{}) string {
	styleCodes := make([]string, len(styles))
	for i, style := range styles {
		styleCodes[i] = fmt.Sprintf("\033[%dm", style)
	}
	combinedStyles := strings.Join(styleCodes, "")
	reset := Reset()
	args := make([]string, len(v))
	for i, arg := range v {
		args[i] = fmt.Sprint(arg)
	}
	text := strings.Join(args, " ")
	return fmt.Sprintf("%s%s%s", combinedStyles, text, reset)
}

// ColorAndStyle formats the text with both color and style
func ColorAndStyle(fg ColorCode, bg ColorCode, style TextStyle, v ...interface{}) string {
	fgCode := OneForeground(fg)
	bgCode := OneBackground(bg)

	styleCode := ""
	if style != 0 {
		styleCode = fmt.Sprintf("\033[%dm", style)
	}

	reset := Reset()

	args := make([]string, len(v))
	for i, arg := range v {
		args[i] = fmt.Sprint(arg)
	}
	text := strings.Join(args, " ")

	return fmt.Sprintf("%s%s%s%s%s", fgCode, bgCode, styleCode, text, reset)
}

// ColorChooseRandom returns a random color code (non-black, including RGB colors).
func ColorChooseRandom() ColorCode {
	colors := []ColorCode{
		Red, Green, Yellow, Blue, Magenta, Cyan, White,
		BrightRed, BrightGreen, BrightYellow, BrightBlue, BrightMagenta, BrightCyan, BrightWhite,
		ColorOrange, ColorPink, ColorPurple, ColorTeal, ColorLimeGreen, ColorIndigo,
	}
	return colors[rand.Intn(len(colors))]
}

// ColorFrom returns a color code based on the given item value.
// Very useful for "cookie" unique identifiers such that related messages are always the same color.
func ColorFrom(item uint64) ColorCode {
	colors := []ColorCode{
		Red,
		Green,
		Yellow,
		Blue,
		Magenta,
		Cyan,
		White,
		BrightRed,
		BrightGreen,
		BrightYellow,
		BrightBlue,
		BrightMagenta,
		BrightCyan,
		BrightWhite,
	}

	// Use the item value to deterministically select a color
	index := uint64(item) % uint64(len(colors))
	return colors[index]
}

// Color formats the given text with the specified foreground and background colors.
func Color(fg, bg ColorCode, v ...interface{}) string {
	fgCode := OneForeground(fg)
	bgCode := OneBackground(bg)
	reset := Reset()
	args := make([]string, len(v))
	for i, arg := range v {
		args[i] = fmt.Sprint(arg)
	}
	text := strings.Join(args, " ")
	return fmt.Sprintf("%s%s%s%s", fgCode, bgCode, text, reset)
}

// Foreground formats the given text with the specified foreground color.
func Foreground(fg ColorCode, v ...interface{}) string {
	fgCode := OneForeground(fg)
	reset := Reset()
	args := make([]string, len(v))
	for i, arg := range v {
		args[i] = fmt.Sprint(arg)
	}
	text := strings.Join(args, " ")
	return fmt.Sprintf("%s%s%s", fgCode, text, reset)
}

// OneForeground returns the ANSI escape sequence for the given color code.
func OneForeground(code ColorCode) string {
	if code.IsRGB() {
		r := (code >> 24) & 0xFF
		g := (code >> 16) & 0xFF
		b := (code >> 8) & 0xFF
		return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
	}
	return fmt.Sprintf("\033[%dm", code)
}

// Background formats the given text with the specified background color.
func Background(code ColorCode, v ...interface{}) string {
	bgCode := OneBackground(code)
	reset := Reset()
	args := make([]string, len(v))
	for i, arg := range v {
		args[i] = fmt.Sprint(arg)
	}
	text := strings.Join(args, " ")
	return fmt.Sprintf("%s%s%s", bgCode, text, reset)
}

// OneBackground returns the ANSI escape sequence for the given background color code.
func OneBackground(code ColorCode) string {
	if code.IsRGB() {
		r := (code >> 24) & 0xFF
		g := (code >> 16) & 0xFF
		b := (code >> 8) & 0xFF
		return fmt.Sprintf("\033[48;2;%d;%d;%dm", r, g, b)
	}
	return fmt.Sprintf("\033[%dm", code+BackgroundOffset)
}

// Reset returns the ANSI escape sequence to reset the text color.
func Reset() string {
	return "\033[0m"
}
