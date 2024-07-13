package main

import (
	"bytes"
	"os"
	"regexp"
	"strings"

	"github.com/wcharczuk/go-chart/v2"

	"github.com/golang/freetype/truetype"
)

func expensesVsIncomeChart() []byte {
	// Load the DejaVu Sans font
	fontBytes, err := os.ReadFile("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
	if err != nil {
		panic(err)
	}
	font, err := truetype.Parse(fontBytes)
	if err != nil {
		panic(err)
	}

	graph := chart.Chart{
		XAxis: chart.XAxis{
			Name: "The XAxis",
		},
		YAxis: chart.YAxis{
			Name: "The YAxis",
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Style: chart.Style{
					StrokeColor: chart.GetDefaultColor(0).WithAlpha(64),
					FillColor:   chart.GetDefaultColor(0).WithAlpha(64),
				},
				XValues: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
				YValues: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			},
		},
		Font: font,
	}

	// TODO remove attempt with "go-chart" renderer
	buffer := bytes.Buffer{}
	err = graph.Render(chart.SVG, &buffer)
	if err != nil {
		panic(err) // Handle error properly
	}
	// Remove line breaks from the SVG content
	svgContent := buffer.String()
	svgContent = strings.ReplaceAll(svgContent, "\n", "")
    svgContent = regexp.MustCompile(`rgba\((\d+),(\d+),(\d+),\d+(\.\d+)?\)`).ReplaceAllString(svgContent, "rgb($1,$2,$3)")
	f, _ := os.Create("output.svg")
	defer f.Close()
	f.WriteString(svgContent)
	return []byte(svgContent)

	// TODO remove attempt with "canvas" renderer
	// f, _ := os.Create("output.svg")
	// defer f.Close()
	// graph.Render(renderers.NewGoChart(renderers.SVG(&svg.Options{
	// 	EmbedFonts: false,
	// })), f)

	// buffer := bytes.Buffer{}
	// err = graph.Render(renderers.NewGoChart(renderers.SVG(&svg.Options{
	// 	EmbedFonts: false,
	// })), &buffer)
	// if err != nil {
	// 	panic(err)
	// }
	// return buffer.Bytes()

	// THIS WORKS START
	// writer := &chart.ImageWriter{}
	// graph.Render(chart.PNG, writer)

	// // Get the image to display:
	// image, err := writer.Image()
	// if err != nil {
	// 	fmt.Println(err) // Handle error properly
	// }
	// return image
}
