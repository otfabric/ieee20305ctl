// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ieee20305 "github.com/otfabric/go-ieee20305"
	ieeeclient "github.com/otfabric/go-ieee20305/client"
	"github.com/otfabric/ieee20305ctl/internal/conn"
)

// The price of each tariff interval is a read of its own: it counts
// against the bound of the command's lists, and stops the command there.
func TestRateComponentBound(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var doc any
		switch r.URL.Path {
		case "/tti":
			var tti ieee20305.TimeTariffInterval
			tti.Href = new("/tti/1")
			tti.ConsumptionTariffIntervalListLink = &ieee20305.ConsumptionTariffIntervalListLink{}
			tti.ConsumptionTariffIntervalListLink.Href = "/tti/1/cti"
			l := &ieee20305.TimeTariffIntervalList{TimeTariffInterval: []ieee20305.TimeTariffInterval{tti}}
			l.All, l.Results = 1, 1
			doc = l
		default:
			l := &ieee20305.ConsumptionTariffIntervalList{}
			doc = l
		}
		data, err := ieee20305.Marshal(doc)
		if err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", ieeeclient.MediaType)
		_, _ = w.Write(data)
	}))
	defer srv.Close()
	c, err := ieeeclient.New(ieeeclient.Config{Server: srv.URL + "/dcap", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	rc := &ieee20305.RateComponent{}
	rc.TimeTariffIntervalListLink.Href = "/tti"

	d := &conn.Device{Client: c}
	if comp, err := rateComponent(context.Background(), d, rc); err != nil || len(comp.Intervals) != 1 {
		t.Fatalf("rateComponent: %+v, %v", comp, err)
	}
	// One read left: the interval, not its price.
	d = &conn.Device{Client: c}
	d.Listed.Store(maxListed - 1)
	if _, err := rateComponent(context.Background(), d, rc); err == nil || !strings.Contains(err.Error(), "more than") {
		t.Errorf("rateComponent at the bound: %v", err)
	}
}
