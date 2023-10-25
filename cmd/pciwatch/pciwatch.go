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

type propRenderer struct {
	title    string
	fn       func(ctx *renderContext) string
	statusFn func(ctx *renderContext) string
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
}, {
	title: "Control",
	fn: func(ctx *renderContext) string {
		return fmt.Sprintf("%04x", ctx.dev.Control)
	},
	statusFn: func(ctx *renderContext) string {
		return ctx.dev.Control.String()
	},
}, {
	title: "Status",
	fn: func(ctx *renderContext) string {
		return fmt.Sprintf("%04x", ctx.dev.Status)
	},
	statusFn: func(ctx *renderContext) string {
		return ctx.dev.Status.String()
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
			tview.NewTableCell(r.title).
				SetReference(r))
	}
	for devIdx, dev := range devs {
		for rowIdx, r := range renderers {
			ctx := renderContext{
				dev: dev,
			}
			table.SetCell(rowIdx, 1+devIdx,
				tview.NewTableCell(r.fn(&ctx)).
					SetReference(ctx))
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

	status := tview.NewTextView().
		SetText("<status>")
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(status, 2, 0, false)

	table.SetSelectionChangedFunc(func(row, column int) {
		statusText := ""
		rnd := table.GetCell(row, 0).GetReference().(propRenderer)
		var cell *tview.TableCell = nil
		if column > 0 {
			cell = table.GetCell(row, column)
		}
		if cell != nil {
			ctx := cell.GetReference().(renderContext)
			if rnd.statusFn != nil {
				statusText = rnd.statusFn(&ctx)
			}
			statusText = fmt.Sprintf("%s - %s\n%s",
				ctx.dev.VendorName,
				ctx.dev.DeviceName,
				statusText,
			)
		}
		status.SetText(statusText)
	})

	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
