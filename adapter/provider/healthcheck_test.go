package provider

import (
	"sync"
	"testing"

	"github.com/metacubex/mihomo/common/utils"
)

func TestRegisterHealthCheckDoesNotWidenStatus(t *testing.T) {
	narrow, err := utils.NewUnsignedRanges[uint16]("204")
	if err != nil {
		t.Fatal(err)
	}
	star, err := utils.NewUnsignedRanges[uint16]("*")
	if err != nil {
		t.Fatal(err)
	}
	if star != nil {
		t.Fatal(`"*" must stay a nil range`)
	}
	url := "https://example.com/generate_204"
	hc := NewHealthCheck(nil, url, 1000, 0, false, narrow)
	defer hc.close()

	hc.registerHealthCheckTask(url, star, "", 0)
	hc.registerHealthCheckTask(url, utils.IntRanges[uint16]{}, "", 0)
	if got := hc.defaultExpectedStatus().String(); got != "204" {
		t.Fatalf("widened to %s", got)
	}

	other, err := utils.NewUnsignedRanges[uint16]("200")
	if err != nil {
		t.Fatal(err)
	}
	hc.registerHealthCheckTask(url, other, "", 0)
	if got := hc.defaultExpectedStatus().String(); got != "200" {
		t.Fatalf("real range = %s", got)
	}

	unset := NewHealthCheck(nil, url, 1000, 0, false, nil)
	defer unset.close()
	unset.registerHealthCheckTask(url, star, "", 0)
	if got := unset.defaultExpectedStatus(); len(got) != 0 {
		t.Fatalf("nil status became %s", got)
	}
	unset.registerHealthCheckTask(url, narrow, "", 0)
	if got := unset.defaultExpectedStatus().String(); got != "204" {
		t.Fatalf("accepted range = %s", got)
	}
	unset.registerHealthCheckTask(url, utils.IntRanges[uint16]{}, "", 0)
	if got := unset.defaultExpectedStatus().String(); got != "204" {
		t.Fatalf("empty range replaced %s", got)
	}
}

func TestRegisterHealthCheckStatusCopyIsStable(t *testing.T) {
	narrow, err := utils.NewUnsignedRanges[uint16]("204")
	if err != nil {
		t.Fatal(err)
	}
	other, err := utils.NewUnsignedRanges[uint16]("200")
	if err != nil {
		t.Fatal(err)
	}
	url := "https://example.com/generate_204"
	hc := NewHealthCheck(nil, url, 1000, 0, false, narrow)
	defer hc.close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			status := hc.defaultExpectedStatus()
			if len(status) == 0 || !status.Check(204) && !status.Check(200) {
				t.Errorf("torn status %v", status)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if i%2 == 0 {
				hc.registerHealthCheckTask(url, other, "", 0)
			} else {
				hc.registerHealthCheckTask(url, narrow, "", 0)
			}
			hc.registerHealthCheckTask(url, utils.IntRanges[uint16]{}, "", 0)
		}
	}()
	wg.Wait()
}
