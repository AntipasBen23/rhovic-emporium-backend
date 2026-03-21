package services

import "math"

type VendorSplitInput struct {
	VendorID    string
	VendorName  string
	VendorOrder string
	LineTotal   int64
	Commission  int64
}

func ResolveCommissionRate(defaultRate float64, vendorOverride, adminRate *float64) float64 {
	rate := defaultRate
	if vendorOverride != nil {
		rate = *vendorOverride
	}
	if adminRate != nil {
		rate = *adminRate
	}
	if rate < 0 {
		return 0
	}
	return rate
}

func CalculateCheckoutAmounts(unitPrice int64, qty float64, rate float64) (lineTotal int64, commission int64, net int64) {
	lineTotal = int64(math.Round(float64(unitPrice) * qty))
	commission = int64(math.Round(float64(lineTotal) * rate))
	net = lineTotal - commission
	return lineTotal, commission, net
}

func AccumulateVendorSummary(summary map[string]*CheckoutVendorSummary, item VendorSplitInput) *CheckoutVendorSummary {
	vs, ok := summary[item.VendorID]
	if !ok {
		vs = &CheckoutVendorSummary{
			VendorID:    item.VendorID,
			VendorName:  item.VendorName,
			VendorOrder: item.VendorOrder,
		}
		summary[item.VendorID] = vs
	}
	vs.Subtotal += item.LineTotal
	vs.Commission += item.Commission
	vs.NetPayable += item.LineTotal - item.Commission
	return vs
}
