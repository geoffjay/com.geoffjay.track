package views

import (
	"fmt"

	"com.geoffjay.track/internal/track"

	"github.com/geoffjay/templ-charts/charts/axes"
	"github.com/geoffjay/templ-charts/charts/bar"
	"github.com/geoffjay/templ-charts/charts/colors"
	"github.com/geoffjay/templ-charts/charts/core"
	"github.com/geoffjay/templ-charts/charts/legends"
	"github.com/geoffjay/templ-charts/charts/line"
	"github.com/geoffjay/templ-charts/charts/render"
	"github.com/geoffjay/templ-charts/charts/scales"
	"github.com/geoffjay/templ-charts/charts/theming"
)

// WeekDay is one day of the weekly bar chart: label plus per-user miles.
type WeekDay struct {
	Label string
	Miles map[string]float64
}

// BuildChartView renders both dashboard charts to SVG strings. Weekly is the
// ordered last-7-days slice; userOrder fixes series order and colors.
func BuildChartView(series map[string][]track.LinePoint, weekly []WeekDay, userOrder []string) (ChartView, error) {
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

// chartTheme bumps text sizes ~27% because the responsive SVG renders at
// ~0.85x inside the mobile card (11px ticks would land at ~9px).
var chartTheme = func() *theming.Theme {
	t := theming.DefaultTheme
	t.Text.FontSize = 14
	t.Axis.Ticks.Text.FontSize = 14
	t.Axis.Legend.Text.FontSize = 14
	t.Legends.Text.FontSize = 14
	return &t
}()

func legendItems(userOrder []string) []legends.Datum {
	scale := colors.GetOrdinalColorScale[any](colors.OrdinalColorScaleConfig{
		Type:   colors.OrdinalTypeScheme,
		Scheme: string(colors.PaletteNivo),
	}, nil)
	items := make([]legends.Datum, 0, len(userOrder))
	for _, u := range userOrder {
		items = append(items, legends.Datum{ID: u, Label: u, Color: scale(u)})
	}
	return items
}

// buildProgressChart renders the cumulative-miles line chart with a legend,
// weekly x ticks, and clean integer y ticks (the axis formatter otherwise
// prints floating-point artifacts like 0.6000000000000001 for cumulative
// sums, and a 30-day point scale crowds every label together).
func buildProgressChart(series map[string][]track.LinePoint, order []string) (string, error) {
	data := make([]line.LineSeries, 0, len(order))
	var labels []string
	for _, u := range order {
		pts := series[u]
		if len(pts) == 0 {
			continue
		}
		pointData := make([]line.LinePointData, 0, len(pts))
		for _, p := range pts {
			pointData = append(pointData, line.LinePointData{X: p.Day.Format("1/2"), Y: p.Miles})
		}
		if u == order[0] {
			labels = make([]string, 0, len(pts))
			for _, p := range pts {
				labels = append(labels, p.Day.Format("1/2"))
			}
		}
		data = append(data, line.LineSeries{ID: u, Data: pointData})
	}

	// Weekly x ticks; the final day is added only when it is not adjacent to
	// the previous tick (a "Sep 20 Sep 21" pair would overlap).
	var xTicks []any
	last := -1
	for i, l := range labels {
		if i%7 == 0 {
			xTicks = append(xTicks, l)
			last = i
		}
	}
	if len(labels) > 0 && len(labels)-1-last >= 4 {
		xTicks = append(xTicks, labels[len(labels)-1])
	}

	// Data max drives both the y domain (one integer of headroom) and the
	// explicit integer tick list.
	maxMiles := 0.0
	for _, s := range data {
		for _, p := range s.Data {
			if m, ok := p.Y.(float64); ok && m > maxMiles {
				maxMiles = m
			}
		}
	}
	area, noPoints := true, false

	// Integer y ticks from 0 up to one above the data max; the axis
	// formatter otherwise prints float artifacts (0.6000000000000001) for
	// cumulative sums, and an auto domain with Nice returns 0.2-steps.
	var yTicks []any
	for v := 0.0; v <= maxMiles+1; v++ {
		yTicks = append(yTicks, v)
	}
	svg, err := render.String(line.Line(line.LineProps{
		Width:        380,
		Height:       300,
		Responsive:   true,
		Data:         data,
		EnableArea:   &area,
		EnablePoints: &noPoints,
		XScale:       scales.ScalePointSpec{},
		YScale:       scales.ScaleLinearSpec{Min: scales.FloatVal(0), Max: scales.FloatVal(maxMiles + 1)},
		Colors:       colors.Set(colors.PaletteNivo).Ordinal(),
		Interactive:  true,
		Theme:        chartTheme,
		AxisBottom:   &axes.AxisProps{TickValues: xTicks},
		AxisLeft:     &axes.AxisProps{TickValues: yTicks},
		Legends: []legends.LegendProps{{
			Anchor:    legends.LegendAnchorTopLeft,
			Direction: legends.LegendDirectionRow,
			Items:     legendItems(order),
		}},
		Margin: core.Margin{Top: 40, Right: 36, Bottom: 40, Left: 44},
	}))
	if err != nil {
		return "", fmt.Errorf("render line: %w", err)
	}
	return svg, nil
}

// buildWeekChart renders grouped per-day bars for the last 7 days with clean
// integer y ticks and a legend.
func buildWeekChart(weekly []WeekDay, order []string) (string, error) {
	if len(weekly) == 0 {
		return "", nil
	}
	data := make([]bar.BarDatum, 0, len(weekly))
	maxMiles := 0.0
	for _, d := range weekly {
		row := map[string]any{"day": d.Label}
		for _, u := range order {
			m := d.Miles[u]
			if m > maxMiles {
				maxMiles = m
			}
			row[u] = m
		}
		data = append(data, row)
	}
	var yTicks []any
	for v := 0.0; v <= maxMiles; v++ {
		yTicks = append(yTicks, v)
	}
	// One x tick per day (the weekday label doubles as the band index).
	dayTicks := make([]any, 0, len(weekly))
	for _, d := range weekly {
		dayTicks = append(dayTicks, d.Label)
	}
	noLabels := false
	svg, err := render.String(bar.Bar(bar.BarProps{
		Width:       380,
		Height:      240,
		IndexBy:     "day",
		Keys:        order,
		Data:        data,
		GroupMode:   bar.GroupModeGrouped,
		Colors:      colors.Set(colors.PaletteNivo).Ordinal(),
		EnableLabel: &noLabels,
		AxisLeft:    &axes.AxisProps{TickValues: yTicks, Legend: "miles"},
		AxisBottom:  &axes.AxisProps{TickValues: dayTicks},
		Theme:       chartTheme,
		Legends: []bar.BarLegendProps{{
			LegendProps: legends.LegendProps{
				Anchor:    legends.LegendAnchorTopLeft,
				Direction: legends.LegendDirectionRow,
				Items:     legendItems(order),
			},
			DataFrom: "keys",
		}},
		Margin: core.Margin{Top: 40, Right: 36, Bottom: 40, Left: 44},
	}))
	if err != nil {
		return "", fmt.Errorf("render bar: %w", err)
	}
	return svg, nil
}
