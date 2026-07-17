package webembed

import (
	"io/fs"
	"testing"
)

func TestEmbeddedWebContainsAllStaticRoutes(t *testing.T) {
	embedded := Embedded()
	for _, name := range []string{
		"index.html",
		"index.txt",
		"calculator.html",
		"calculator.txt",
		"assets.html",
		"assets.txt",
		"data.html",
		"data/imports/_.html",
		"data/imports/_.txt",
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
