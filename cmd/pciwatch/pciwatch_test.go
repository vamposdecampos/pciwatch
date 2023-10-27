package main

import  (
	"encoding/json"
	"os"
	"fmt"
	"testing"

	"github.com/u-root/u-root/pkg/pci"

)

func loadJson(fname string) (pci.Devices, error) {
	buf, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	var devs pci.Devices
	err = json.Unmarshal(buf, &devs)
	if err != nil {
		return nil, err
	}
	return devs, nil
}

func TestParseCaps(t *testing.T) {
	devs, err := loadJson("testdata/pci.json")
	if err != nil {
		t.Errorf("json error: %q", err)
	}
	for _, dev := range(devs) {
		ctx := renderContext{
			dev: dev,
		}
		ctx.ParseCaps()
		fmt.Printf("%#v\n", ctx)
		var exp CapExpress
		ctx.GetExpressCaps(&exp)
		fmt.Printf("exp: %#v\n", exp)
	}
}
