package main

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func runUI() {
	a := app.New()
	w := a.NewWindow("TODO App")
	// TODO remove
	w.Resize(fyne.NewSize(800, 600))
	w.SetContent(content())
	w.ShowAndRun()
}

func content() fyne.CanvasObject {
	c := container.NewBorder(
		nil, // TOP
		nil, // BOTTOM
		nil, // LEFT
		nil, // RIGHT
		container.NewVScroll(
			container.NewVBox(
				expensesVsIncome(),
				perCategory(),
				monthly(),
				widget.NewLabel("-- bottom --"),
			),
		),
	)
	return c
}

func expensesVsIncome() fyne.CanvasObject {
	// rect := canvas.NewRectangle(color.RGBA{100, 100, 0, 255})
	// rect.SetMinSize(fyne.NewSize(100, 200))

	// expensesVsIncomeSvg, err := fyne.LoadResourceFromPath("output.svg")
	// if err!= nil {
    //     fmt.Println("Error loading SVG:", err)
    //     return nil
    // }

	svgBytes := expensesVsIncomeChart()
	expensesVsIncomeSvg := &fyne.StaticResource{
		StaticName:  "expensesVsIncome.svg",
        StaticContent: svgBytes,
	}
	image := canvas.NewImageFromResource(expensesVsIncomeSvg)
	// image.FillMode = canvas.ImageFillStretch
	image.SetMinSize(fyne.NewSize(0, 200))

	// image := expensesVsIncomeChart()
	container := container.NewVBox(
		container.NewCenter(
			widget.NewLabel("Expenses vs Income"),
		),
		image,
		// rect,
	)
	return container
}

func perCategory() fyne.CanvasObject {
	expenses := canvas.NewRectangle(color.RGBA{255, 100, 0, 255})
	expenses.SetMinSize(fyne.NewSize(0, 300)) // Width will be managed by the grid.
	income := canvas.NewRectangle(color.RGBA{0, 100, 0, 255})
	income.SetMinSize(fyne.NewSize(0, 300)) // Width will be managed by the grid.
	return container.NewVBox(
		container.NewGridWithColumns(
			2,
			container.NewVBox(
				container.NewCenter(
					widget.NewLabel("Total Expenses per Category"),
				),
				expenses,
			),
			container.NewVBox(
				container.NewCenter(
					widget.NewLabel("Total Income per Category"),
				),
				income,
			),
		),
	)
}

func monthly() fyne.CanvasObject {
	// Build rows for each month.
	monthRows := make([]fyne.CanvasObject, 5)
	for i := range monthRows {
		graph := canvas.NewRectangle(color.RGBA{0, 0, 255, 255})
		graph.SetMinSize(fyne.NewSize(0, 150)) // Width will be managed by the grid.
		monthRows[i] = container.NewVBox(
			widget.NewLabel(fmt.Sprintf("Month %d", i+1)),
			graph,
		)
	}
	// Build container for all rows.
	// Add
	return container.NewVBox(
		container.NewCenter(
			container.NewCenter(
				widget.NewLabel("Monthly Expenses per Category"),
			),
		),
		container.NewVBox(
			monthRows...,
		),
	)
}
