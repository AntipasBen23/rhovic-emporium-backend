package tests

import (
	"testing"

	"rhovic/backend/internal/services"
)

func TestResolveCommissionRate_PreferenceOrder(t *testing.T) {
	vendor := 0.12
	admin := 0.18

	if got := services.ResolveCommissionRate(0.10, &vendor, &admin); got != 0.18 {
		t.Fatalf("expected admin override to win, got %v", got)
	}

	if got := services.ResolveCommissionRate(0.10, &vendor, nil); got != 0.12 {
		t.Fatalf("expected vendor override to win when admin override is nil, got %v", got)
	}

	if got := services.ResolveCommissionRate(0.10, nil, nil); got != 0.10 {
		t.Fatalf("expected default rate, got %v", got)
	}

	negative := -0.40
	if got := services.ResolveCommissionRate(0.10, nil, &negative); got != 0 {
		t.Fatalf("expected negative rate to clamp to zero, got %v", got)
	}
}

func TestAccumulateVendorSummary_GroupsItemsByVendor(t *testing.T) {
	summary := map[string]*services.CheckoutVendorSummary{}

	first := services.AccumulateVendorSummary(summary, services.VendorSplitInput{
		VendorID:    "vendor-a",
		VendorName:  "Vendor A",
		VendorOrder: "order-a",
		LineTotal:   20000,
		Commission:  2000,
	})
	second := services.AccumulateVendorSummary(summary, services.VendorSplitInput{
		VendorID:    "vendor-a",
		VendorName:  "Vendor A",
		VendorOrder: "order-a-later-ignored",
		LineTotal:   5000,
		Commission:  500,
	})
	services.AccumulateVendorSummary(summary, services.VendorSplitInput{
		VendorID:    "vendor-b",
		VendorName:  "Vendor B",
		VendorOrder: "order-b",
		LineTotal:   15000,
		Commission:  1500,
	})

	if len(summary) != 2 {
		t.Fatalf("expected 2 vendors in summary, got %d", len(summary))
	}
	if first != second {
		t.Fatalf("expected repeated vendor items to accumulate on same summary pointer")
	}

	gotA := summary["vendor-a"]
	if gotA.Subtotal != 25000 || gotA.Commission != 2500 || gotA.NetPayable != 22500 {
		t.Fatalf("unexpected vendor-a summary: %+v", gotA)
	}
	if gotA.VendorOrder != "order-a" {
		t.Fatalf("expected first vendor order id to be kept, got %q", gotA.VendorOrder)
	}

	gotB := summary["vendor-b"]
	if gotB.Subtotal != 15000 || gotB.Commission != 1500 || gotB.NetPayable != 13500 {
		t.Fatalf("unexpected vendor-b summary: %+v", gotB)
	}
}

func TestCalculateCheckoutAmounts_ComputesRoundedTotals(t *testing.T) {
	lineTotal, commission, net := services.CalculateCheckoutAmounts(8500, 2.5, 0.1)

	if lineTotal != 21250 {
		t.Fatalf("expected line total 21250, got %d", lineTotal)
	}
	if commission != 2125 {
		t.Fatalf("expected commission 2125, got %d", commission)
	}
	if net != 19125 {
		t.Fatalf("expected net 19125, got %d", net)
	}
}
