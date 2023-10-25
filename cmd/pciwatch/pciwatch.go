package main

import (
	"fmt"
	"log"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/u-root/u-root/pkg/pci"
)

type renderContext struct {
	dev *pci.PCI
}

type propRenderFunc func(ctx *renderContext) string

type propRenderer struct {
	title string
	fn    propRenderFunc
}

var renderers = []propRenderer{{
	title: "B:D.F",
	fn: func(ctx *renderContext) string {
		return ctx.dev.Addr
	},
}, {
	title: "IDs",
	fn: func(ctx *renderContext) string {
		return fmt.Sprintf("%04X:%04x", ctx.dev.Vendor, ctx.dev.Device)
	},
}}

func main() {
	reader, err := pci.NewBusReader()
	if err != nil {
		log.Fatal(err)
	}
	devs, err := reader.Read() // TODO: filter
	if err != nil {
		log.Fatal(err)
	}

	app := tview.NewApplication()
	table := tview.NewTable()

	for rowIdx, r := range renderers {
		table.SetCell(rowIdx, 0,
			tview.NewTableCell(r.title))
	}
	for devIdx, dev := range devs {
		for rowIdx, r := range renderers {
			ctx := renderContext{
				dev: dev,
			}
			table.SetCell(rowIdx, 1+devIdx,
				tview.NewTableCell(r.fn(&ctx)))
		}
	}

	table.Select(0, 1)
	table.SetFixed(1, 1)
	table.SetSelectable(true, true)
	// TODO column select? table.SetSelectable(false, true)

	table.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			app.Stop()
		}
	})
	if err := app.SetRoot(table, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
