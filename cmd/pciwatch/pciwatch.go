package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

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

func ternS(value bool, trueS, falseS string) string {
	if (value) {
		return trueS;
	} else {
		return falseS;
	}
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
		return fmt.Sprintf("%04x:%04x", ctx.dev.Vendor, ctx.dev.Device)
	},
}, {
	title: "Sec",
	fn: func(ctx *renderContext) string {
		if !ctx.dev.Bridge {
			return ""
		}
		res := fmt.Sprintf("%02x", ctx.dev.Secondary)
		if ctx.dev.Subordinate != ctx.dev.Secondary {
			res += fmt.Sprintf("-%02x", ctx.dev.Subordinate)
		}
		return res
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
	cellFn: func(ctx *renderContext, cell *tview.TableCell) {
		brctl := binary.LittleEndian.Uint16(ctx.dev.Config[BridgeControl:BridgeControl+2])
		if brctl & 0x40 != 0 {
			cell.SetTextColor(tcell.ColorRed)
		} else {
			cell.SetTextColor(tcell.ColorDefault)
		}
	},
/*
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
*/
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
	title: "  Errors",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		devsta := ctx.expCap.DevSta
		return ternS(devsta & 1 != 0, "c", " ") +
			ternS(devsta & 2 != 0, "n", " ") +
			ternS(devsta & 4 != 0, "f", " ") +
			ternS(devsta & 8 != 0, "u", " ") +
			// not errors:
			ternS(devsta & 0x10 != 0, "x", " ") +
			ternS(devsta & 0x20 != 0, "t", " ");
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
	title: "  DLActive",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		if ctx.expCap.LnkSta & 0x2000 != 0 {
			return "+"
		} else {
			return "-"
		}
	},
	cellFn: func(ctx *renderContext, cell *tview.TableCell) {
		if ctx.expCap.LnkSta & 0x2000 != 0 {
			cell.SetTextColor(tcell.ColorGreen)
		} else {
			cell.SetTextColor(tcell.ColorRed)
		}
	},
}, {
	title: "  Speed",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return fmt.Sprintf("%d", ctx.expCap.LnkSta & 0xf)
	},
}, {
	title: "  Width",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return fmt.Sprintf("%d", (ctx.expCap.LnkSta & 0x3f0) >> 4)
	},
}, {
	title: "LnkCtl",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return fmt.Sprintf("%04x", ctx.expCap.LnkCtl)
	},
	cellFn: func(ctx *renderContext, cell *tview.TableCell) {
		if ctx.expCap.LnkCtl & 0x10 != 0 {
			cell.SetTextColor(tcell.ColorRed)
		} else {
			cell.SetTextColor(tcell.ColorDefault)
		}
	},
}, {
	title: "LnkSta2",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return fmt.Sprintf("%04x", ctx.expCap.LnkSta2)
	},
}, {
	title: "LnkCtl2",
	fn: func(ctx *renderContext) string {
		if !ctx.HasCaps() {
			return ""
		}
		return fmt.Sprintf("%04x", ctx.expCap.LnkCtl2)
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
}}

// BRIDGE_CONTROL=40:40
func (ctx *renderContext) ToggleSBR() {
	if *readJSON != "" {
		return // nop
	}
	// TODO: errors
	brctl, _ := ctx.dev.ReadConfigRegister(BridgeControl, 16)
	brctl ^= 0x40
	ctx.dev.WriteConfigRegister(BridgeControl, 16, brctl)
}

// CAP_EXP+10.w=10:10
func (ctx *renderContext) ToggleLink() {
	if *readJSON != "" {
		return // nop
	}
	off, ok := ctx.capOffset[CapIdExp]
	if !ok {
		return
	}
	// TODO: errors
	lnkctl, _ := ctx.dev.ReadConfigRegister(int64(off) + 0x10, 16)
	lnkctl ^= 0x10
	ctx.dev.WriteConfigRegister(int64(off) + 0x10, 16, lnkctl)
}

// CAP_EXP+10.w=20:20
func (ctx *renderContext) Retrain() {
	if *readJSON != "" {
		return // nop
	}
	off, ok := ctx.capOffset[CapIdExp]
	if !ok {
		return
	}
	// TODO: errors
	lnkctl, _ := ctx.dev.ReadConfigRegister(int64(off) + 0x10, 16)
	lnkctl |= 0x20
	ctx.dev.WriteConfigRegister(int64(off) + 0x10, 16, lnkctl)
}

// CAP_EXP+30.w=10:10
func (ctx *renderContext) EnterCompliance(on bool) {
	if *readJSON != "" {
		return // nop
	}
	off, ok := ctx.capOffset[CapIdExp]
	if !ok {
		return
	}
	// TODO: check for express v2
	// TODO: errors
	lnkctl2, _ := ctx.dev.ReadConfigRegister(int64(off) + 0x30, 16)
	lnkctl2 |= 0x10
	if !on {
		lnkctl2 ^= 0x10
	}
	ctx.dev.WriteConfigRegister(int64(off) + 0x30, 16, lnkctl2)
}

var (
	filterRE  = flag.String("r", ".*", "Regex to filter devices")
	readJSON  = flag.String("J", "", "Read JSON in instead of /sys")
	horizontal = flag.Bool("H", false, "Horizontal layout (devices in columns)")
)

func cellRow(propIdx, devIdx int) int {
	if *horizontal {
		return propIdx
	} else {
		return devIdx
	}
}

func cellCol(propIdx, devIdx int) int {
	if *horizontal {
		return devIdx
	} else {
		return propIdx
	}
}

var filter *regexp.Regexp

func filterDevs(p *pci.PCI) bool {
	slug := fmt.Sprintf("%s v%04x d%04x c%08x",
		p.Addr,
		p.Vendor,
		p.Device,
		p.Class,
	)
	return filter.MatchString(slug)
}

func main() {
	flag.Parse()
	filter = regexp.MustCompile(*filterRE)

	var devs pci.Devices
	if len(*readJSON) == 0 {
		reader, err := pci.NewBusReader()
		if err != nil {
			log.Fatal(err)
		}
		devs, err = reader.Read(filterDevs)
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
	status := tview.NewTextView().
		SetText("<status>")
	fps := tview.NewTextView().SetText("<fps>")

	for idx, r := range renderers {
		title := r.title
		if !*horizontal {
			title = strings.TrimSpace(title)
		}
		table.SetCell(
			cellRow(idx, 0),
			cellCol(idx, 0),
			tview.NewTableCell(title).
				// N.B. do not attempt &r
				SetReference(&renderers[idx]))
	}

	devRank := func(bdf string) int {
		var maxDev int
		if *horizontal {
			maxDev = table.GetColumnCount()
		} else {
			maxDev = table.GetRowCount()
		}
		for i := 1; i < maxDev; i++ {
			// FIXME: assumes B:D.F is first renderer
			cell := table.GetCell(cellRow(0, i), cellCol(0, i))
			if cell.Text == bdf {
				return i
			}
			if cell.Text > bdf {
				if *horizontal {
					table.InsertColumn(i)
				} else {
					table.InsertRow(i)
				}
				return i
			}
		}
		return maxDev
	}

	go (func() {
		for {
			//time.Sleep(time.Millisecond * 100)
			// TODO: sleep?
			t1 := time.Now()
			for _, dev := range devs {
				ctx := renderContext{
					dev: dev,
				}
				ctx.ParseCaps()
				ctx.GetExpressCaps(&ctx.expCap)
				app.QueueUpdateDraw(func() {
					devIdx := devRank(dev.Addr)
					for rndIdx, r := range renderers {
						cell := tview.NewTableCell(r.fn(&ctx)).
							SetReference(&ctx)
						if r.cellFn != nil {
							r.cellFn(&ctx, cell)
						}
						table.SetCell(
							cellRow(rndIdx, devIdx),
							cellCol(rndIdx, devIdx),
							cell)
					}
				});
			}
			if len(*readJSON) != 0 {
				return
			}
			// TODO: errors
			devs.ReadConfig()
			delta := time.Since(t1)
			app.QueueUpdateDraw(func() {
				fps.SetText(fmt.Sprintf("%v", delta.Round(time.Millisecond)))
			});
		}
	})();

	table.Select(cellRow(0, 1), cellCol(0, 1))
	table.SetFixed(1, 1)
	table.SetSelectable(true, true)
	// TODO column select? table.SetSelectable(false, true)

	table.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			app.Stop()
		}
	})

	hbox := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(status, 0, 1, true).
		AddItem(fps, 10, 0, false)
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(hbox, 2, 0, false)

	table.SetSelectionChangedFunc(func(row, column int) {
		var rndCell *tview.TableCell
		if *horizontal {
			rndCell = table.GetCell(row, 0)
		} else {
			rndCell = table.GetCell(0, column)
		}
		statusText := ""
		rnd := rndCell.GetReference().(*propRenderer)
		cell := table.GetCell(row, column)
		if *horizontal && column == 0 {
			cell = nil
		}
		if !*horizontal && row == 0 {
			cell = nil
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

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if (event.Key() == tcell.KeyRune) {
			r, c := table.GetSelection()
			cell := table.GetCell(r, c)
			if *horizontal && c == 0 {
				cell = nil
			}
			if !*horizontal && r == 0 {
				cell = nil
			}
			if cell == nil {
				// nothing to do
				return event;
			}
			ctx := cell.GetReference().(*renderContext)
			switch event.Rune() {
			case 'R':
				ctx.ToggleSBR()
			case 'L':
				ctx.ToggleLink()
			case 'r':
				ctx.Retrain()
			case 'C':
				ctx.EnterCompliance(true)
			case 'c':
				ctx.EnterCompliance(false)
			case 'Q':
				app.Stop();
			}
		}
		return event;
	});

	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
