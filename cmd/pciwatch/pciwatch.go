package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/u-root/u-root/pkg/pci"
	"github.com/lunixbochs/struc"
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
	expCap CapExpress
}

type propRenderer struct {
	title    string
	fn       func(ctx *renderContext) string
	statusFn func(ctx *renderContext) string
	cellFn   func(ctx *renderContext, cell *tview.TableCell)
}


const (
	// Status bits
	StatusCapList	= 0x10
	// offsets
	CapabilityList = 0x34
	BridgeControl = 0x3e
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

type CapExpress struct {
	Caps uint16 `struc:"little,uint16"`
	DevCap uint32 `struc:"little,uint32"`
	DevCtl uint16 `struc:"little,uint16"`
	DevSta uint16 `struc:"little,uint16"`
	LnkCap uint32 `struc:"little,uint32"`
	LnkCtl uint16 `struc:"little,uint16"`
	LnkSta uint16 `struc:"little,uint16"`
	SltCap uint32 `struc:"little,uint32"`
	SltCtl uint16 `struc:"little,uint16"`
	SltSta uint16 `struc:"little,uint16"`
	RootCtl uint16 `struc:"little,uint16"`
	RootCap uint16 `struc:"little,uint16"`
	RootSta uint32 `struc:"little,uint32"`
	DevCap2 uint32 `struc:"little,uint32"`
	DevCtl2 uint16 `struc:"little,uint16"`
	DevSta2 uint16 `struc:"little,uint16"`
	LnkCap2 uint32 `struc:"little,uint32"`
	LnkCtl2 uint16 `struc:"little,uint16"`
	LnkSta2 uint16 `struc:"little,uint16"`
}

func (r *renderContext) GetExpressCaps(cap *CapExpress) error {
	off, ok := r.capOffset[CapIdExp]
	if !ok {
		return fmt.Errorf("no express capability")
	}
	return struc.Unpack(bytes.NewReader(r.dev.Config[off+2:]), cap)
}

var renderers = []propRenderer{{
	title: "B:D.F",
	fn: func(ctx *renderContext) string {
		return ctx.dev.Addr
	},
	cellFn: func(ctx *renderContext, cell *tview.TableCell) {
		if ctx.dev.Bridge {
			cell.SetTextColor(tcell.ColorBlue)
		}
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
}, {
	title: "BrCtl",
	fn: func(ctx *renderContext) string {
		if !ctx.dev.Bridge {
			return ""
		}
		brctl := binary.LittleEndian.Uint16(ctx.dev.Config[BridgeControl:BridgeControl+2])
		return fmt.Sprintf("%04x", brctl)
	},
}, {
	title: "Caps",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return "aye"
	},
	statusFn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return fmt.Sprintf("%#v", ctx.capOffset)
	},
}, {
	title: "DevSta",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return fmt.Sprintf("%04x", ctx.expCap.DevSta)
	},
	statusFn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return fmt.Sprintf("%#v", ctx.expCap)
	},
}, {
	title: "LnkSta",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return fmt.Sprintf("%04x", ctx.expCap.LnkSta)
	},
}, {
	title: "SltSta",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return fmt.Sprintf("%04x", ctx.expCap.SltSta)
	},
}, {
	title: "RootSta",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return fmt.Sprintf("%08x", ctx.expCap.RootSta)
	},
}, {
	title: "DevSta2",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return fmt.Sprintf("%04x", ctx.expCap.DevSta2)
	},
}, {
	title: "LnkSta2",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return fmt.Sprintf("%04x", ctx.expCap.LnkSta2)
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
				// N.B. do not attempt &r
				SetReference(&renderers[rowIdx]))
	}
	for devIdx, dev := range devs {
		for rowIdx, r := range renderers {
			ctx := renderContext{
				dev: dev,
			}
			ctx.ParseCaps()
			ctx.GetExpressCaps(&ctx.expCap)
			cell := tview.NewTableCell(r.fn(&ctx)).
				SetReference(&ctx)
			if r.cellFn != nil {
				r.cellFn(&ctx, cell)
			}
			table.SetCell(rowIdx, 1+devIdx, cell)
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
		rnd := table.GetCell(row, 0).GetReference().(*propRenderer)
		var cell *tview.TableCell = nil
		if column > 0 {
			cell = table.GetCell(row, column)
		}
		if cell != nil {
			ctx := cell.GetReference().(*renderContext)
			if rnd.statusFn != nil {
				statusText = rnd.statusFn(ctx)
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
