package webembed

import (
	"io/fs"
	"testing"
)

func TestEmbeddedWebContainsAllStaticRoutes(t *testing.T) {
	embedded := Embedded()
	for _, name := range []string{
		"index.html",
		"calculator.html",
		"data.html",
		"data/imports/_.html",
		"neighborhoods.html",
		"action-window.html",
		"methods.html",
		"methods/listings-up-transactions-weak.html",
		"methods/asking-price-vs-transactions.html",
		"methods/buyer-window.html",
		"methods/more-price-cuts.html",
		"methods/upgrade-price-gap.html",
		"methods/monthly-payment-safety.html",
		"methods/old-home-sale-delay.html",
		"templates.html",
		"watchlist.html",
		"icon.svg",
	} {
		if _, err := fs.Stat(embedded, name); err != nil {
			t.Errorf("%s missing from embedded web fs: %v", name, err)
		}
	}
}
