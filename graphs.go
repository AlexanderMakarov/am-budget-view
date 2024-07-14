package main

import (
	"image/color"

	// "github.com/wcharczuk/go-chart/v2"

	// "github.com/golang/freetype/truetype"
	"flag"

	"github.com/ajstarks/fc"
	"github.com/ajstarks/fc/chart"
)

// func expensesVsIncomeChart() []byte {
// 	// Load the DejaVu Sans font
// 	fontBytes, err := os.ReadFile("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
// 	if err != nil {
// 		panic(err)
// 	}
// 	font, err := truetype.Parse(fontBytes)
// 	if err != nil {
// 		panic(err)
// 	}

// 	graph := chart.Chart{
// 		XAxis: chart.XAxis{
// 			Name: "The XAxis",
// 		},
// 		YAxis: chart.YAxis{
// 			Name: "The YAxis",
// 		},
// 		Series: []chart.Series{
// 			chart.ContinuousSeries{
// 				Style: chart.Style{
// 					StrokeColor: chart.GetDefaultColor(0).WithAlpha(64),
// 					FillColor:   chart.GetDefaultColor(0).WithAlpha(64),
// 				},
// 				XValues: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
// 				YValues: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
// 			},
// 		},
// 		Font: font,
// 	}

// 	// TODO remove attempt with "go-chart" renderer
// 	buffer := bytes.Buffer{}
// 	err = graph.Render(chart.SVG, &buffer)
// 	if err != nil {
// 		panic(err) // Handle error properly
// 	}
// 	// Remove line breaks from the SVG content
// 	svgContent := buffer.String()
// 	svgContent = strings.ReplaceAll(svgContent, "\n", "")
//     svgContent = regexp.MustCompile(`rgba\((\d+),(\d+),(\d+),\d+(\.\d+)?\)`).ReplaceAllString(svgContent, "rgb($1,$2,$3)")
// 	f, _ := os.Create("output.svg")
// 	defer f.Close()
// 	f.WriteString(svgContent)
// 	return []byte(svgContent)

// 	// TODO remove attempt with "canvas" renderer
// 	// f, _ := os.Create("output.svg")
// 	// defer f.Close()
// 	// graph.Render(renderers.NewGoChart(renderers.SVG(&svg.Options{
// 	// 	EmbedFonts: false,
// 	// })), f)

// 	// buffer := bytes.Buffer{}
// 	// err = graph.Render(renderers.NewGoChart(renderers.SVG(&svg.Options{
// 	// 	EmbedFonts: false,
// 	// })), &buffer)
// 	// if err != nil {
// 	// 	panic(err)
// 	// }
// 	// return buffer.Bytes()

// 	// THIS WORKS START
// 	// writer := &chart.ImageWriter{}
// 	// graph.Render(chart.PNG, writer)

// 	// // Get the image to display:
// 	// image, err := writer.Image()
// 	// if err != nil {
// 	// 	fmt.Println(err) // Handle error properly
// 	// }
// 	// return image
// }

func fcChart(width, height int) *fc.Canvas {
	chart := chart.ChartBox{
		Title: "Expenses vs Income",
		Data: []chart.NameValue{
			{Label: "1.0", Value: 1.0},
			{Label: "2.0", Value: 2.0},
			{Label: "3.0", Value: 4.0},
			{Label: "4.0", Value: 4.0},
			{Label: "5.0", Value: 5.0},
		},
		Minvalue:  1.0, // TODO need to calculate it!
		Maxvalue:  2.0, // TODO need to calculate it!
		Color:     color.RGBA{0, 0, 0, 255},
		Left:      10,
		Right:     90,
		Top:       90,
		Bottom:    50,
		Zerobased: true,
	}

	// Draw canvas.
	canvas := fc.NewCanvas("Canvas name", width, height)

	// Define the colors
	var dcolor string
	flag.StringVar(&dcolor, "color", "steelblue", "color")
	datacolor := fc.ColorLookup(dcolor)
	labelcolor := color.RGBA{100, 100, 100, 255}
	bgcolor := color.RGBA{255, 255, 255, 255}
	canvas.Rect(0, 0, 100, 100, bgcolor)
	chart.Color = datacolor
	chart.Color = labelcolor
	return &canvas
}

func (_ *fc.Canvas) Hide() {

}
