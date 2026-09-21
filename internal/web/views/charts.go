package views

import (
	"fmt"

	"com.geoffjay.track/internal/track"

	"github.com/geoffjay/templ-charts/charts/bar"
	"github.com/geoffjay/templ-charts/charts/colors"
	"github.com/geoffjay/templ-charts/charts/core"
	"github.com/geoffjay/templ-charts/charts/line"
	"github.com/geoffjay/templ-charts/charts/render"
	"github.com/geoffjay/templ-charts/charts/scales"
)

// BuildChartView renders both dashboard charts to SVG strings. Weekly maps
// day-label -> user -> miles for the current week's grouped bars.
func BuildChartView(series map[string][]track.LinePoint, weekly map[string]map[string]float64, userOrder []string) (ChartView, error) {
	progress, err := buildProgressChart(series, userOrder)
	if err != nil {
		return ChartView{}, err
	}
	week, err := buildWeekChart(weekly, userOrder)
	if err != nil {
		return ChartView{}, err
	}
	return ChartView{ProgressSVG: progress, WeekSVG: week}, nil
}

// buildProgressChart renders the cumulative-miles-over-time line chart.
func buildProgressChart(series map[string][]track.LinePoint, order []string) (string, error) {
	data := make([]line.LineSeries, 0, len(order))
	for _, u := range order {
		pts := series[u]
		if len(pts) == 0 {
			continue
		}
		pointData := make([]line.LinePointData, 0, len(pts))
		for _, p := range pts {
			pointData = append(pointData, line.LinePointData{X: p.Day.Format("Jan 2"), Y: p.Miles})
		}
		data = append(data, line.LineSeries{ID: u, Data: pointData})
	}
	area, points := true, true
	svg, err := render.String(line.Line(line.LineProps{
		Width:        640,
		Height:       320,
		Responsive:   true,
		Data:         data,
		EnableArea:   &area,
		EnablePoints: &points,
		// Point scale over days: equal spacing, no invented empty days.
		XScale:     scales.ScalePointSpec{},
		YScale:     scales.ScaleLinearSpec{Min: scales.FloatVal(0), Max: scales.AutoFloat()},
		Colors:     colors.Set(colors.PaletteNivo).Ordinal(),
		Interactive: true,
		Margin:     core.Margin{Top: 20, Right: 20, Bottom: 40, Left: 50},
	}))
	if err != nil {
		return "", fmt.Errorf("render line: %w", err)
	}
	return svg, nil
}

// buildWeekChart renders the grouped per-day bar chart for the last 7 days.
func buildWeekChart(weekly map[string]map[string]float64, order []string) (string, error) {
	if len(weekly) == 0 {
		return "", nil
	}
	data := make([]bar.BarDatum, 0, len(weekly))
	for day, byUser := range weekly {
		row := map[string]any{"day": day}
		for _, u := range order {
			row[u] = byUser[u]
		}
		data = append(data, row)
	}
	svg, err := render.String(bar.Bar(bar.BarProps{
		Width:     640,
		Height:    260,
		IndexBy:   "day",
		Keys:      order,
		Data:      data,
		GroupMode: bar.GroupModeGrouped,
		Colors:    colors.Set(colors.PaletteNivo).Ordinal(),
	}))
	if err != nil {
		return "", fmt.Errorf("render bar: %w", err)
	}
	return svg, nil
}