package badge

import (
	"fmt"
	"html"
)

// Status badge colors (shields.io style)
const (
	ColorOK   = "#4c1"   // Green
	ColorLate = "#dfb317" // Yellow/Orange
	ColorDown = "#e05d44" // Red
	ColorGray = "#9f9f9f" // Unknown/Not found
)

// Generate creates an SVG badge for a monitor status
func Generate(name, status string) string {
	color := getColor(status)
	statusText := status
	if status == "" {
		statusText = "unknown"
	}

	nameWidth := len(name)*6 + 12
	statusWidth := len(statusText)*6 + 12
	totalWidth := nameWidth + statusWidth

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img">
  <title>%s: %s</title>
  <linearGradient id="s" x2="0" y2="100%%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <clipPath id="r">
    <rect width="%d" height="20" rx="3" fill="#fff"/>
  </clipPath>
  <g clip-path="url(#r)">
    <rect width="%d" height="20" fill="#555"/>
    <rect x="%d" width="%d" height="20" fill="%s"/>
    <rect width="%d" height="20" fill="url(#s)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="110">
    <text aria-hidden="true" x="%d" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)">%s</text>
    <text x="%d" y="140" transform="scale(.1)">%s</text>
    <text aria-hidden="true" x="%d" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)">%s</text>
    <text x="%d" y="140" transform="scale(.1)">%s</text>
  </g>
</svg>`,
		totalWidth,
		html.EscapeString(name), html.EscapeString(statusText),
		totalWidth,
		nameWidth,
		nameWidth, statusWidth, color,
		totalWidth,
		(nameWidth*10)/2, html.EscapeString(name),
		(nameWidth*10)/2, html.EscapeString(name),
		nameWidth*10+(statusWidth*10)/2, html.EscapeString(statusText),
		nameWidth*10+(statusWidth*10)/2, html.EscapeString(statusText),
	)
}

// GenerateSimple creates a simpler, smaller badge
func GenerateSimple(name, status string) string {
	color := getColor(status)
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="20">
  <rect width="100" height="20" rx="3" fill="%s"/>
  <text x="50" y="14" fill="#fff" font-family="sans-serif" font-size="11" text-anchor="middle">%s</text>
</svg>`, color, html.EscapeString(status))
}

func getColor(status string) string {
	switch status {
	case "OK":
		return ColorOK
	case "LATE":
		return ColorLate
	case "DOWN":
		return ColorDown
	default:
		return ColorGray
	}
}
