package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/u-root/u-root/pkg/pci"
)

type capId uint8

const (
	CapIdNull capId = iota
	CapIdPm
	CapIdAgp
	CapIdVpd
	CapIdSlotId
	CapIdMsi
	CapIdChswp
	CapIdPcix
	CapIdHt
	CapIdVndr
	CapIdDbg
	CapIdCcrc
	CapIdHotplug
	CapIdSsvid
	CapIdAgp3
	CapIdSecure
	CapIdExp
	CapIdMsix
	CapIdSata
	CapIdAf
	CapIdEa
)

type extCapId uint16

const (
	ExtCapIdNull extCapId = iota
)

type renderContext struct {
	dev *pci.PCI
	capOffset map[capId]uint8
	extCapOffset map[extCapId]uint32
}

type propRenderer struct {
	title    string
	fn       func(ctx *renderContext) string
	statusFn func(ctx *renderContext) string
}


const (
	// Status bits
	StatusCapList	= 0x10
	// offsets
	CapabilityList = 0x34
)

// TODO: move to pci.PCI
func (r *renderContext) HasCaps() bool {
	return r.dev.Status & StatusCapList != 0
}

func (r *renderContext) ParseCaps() {
	r.capOffset = make(map[capId]uint8)
	if !r.HasCaps() {
		return
	}
	var offset uint8 = r.dev.Config[CapabilityList]
	seen := make(map[uint8]bool)
	for {
		offset = offset & 0xfc
		if offset == 0 {
			return
		}
		_, seenBefore := seen[offset]
		if seenBefore {
			// TODO: error
			break
		}
		seen[offset] = true

		if int(offset) + 1 >= len(r.dev.Config) {
			// TODO: error
			break
		}
		id := r.dev.Config[offset]
		if id == 0xff {
			break
		}
		next := r.dev.Config[offset + 1]
		r.capOffset[capId(id)] = offset
		offset = next
	}
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

var (
	readJSON  = flag.String("J", "", "Read JSON in instead of /sys")
)

func main() {
	flag.Parse()

	var devs pci.Devices
	if len(*readJSON) == 0 {
		reader, err := pci.NewBusReader()
		if err != nil {
			log.Fatal(err)
		}
		devs, err = reader.Read() // TODO: filter
		if err != nil {
			log.Fatal(err)
		}
	} else {
		buf, err := os.ReadFile(*readJSON)
		if err != nil {
			log.Fatal(err)
		}
		err = json.Unmarshal(buf, &devs)
		if err != nil {
			log.Fatal(err)
		}
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
