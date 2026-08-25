package qrgen

import qrcode "github.com/skip2/go-qrcode"

// RecoveryLevel re-exports the underlying error-correction type so callers
// don't need to import the qrcode library directly.
type RecoveryLevel = qrcode.RecoveryLevel

const (
	ECLow      = qrcode.Low     // ~7% recovery
	ECMedium   = qrcode.Medium  // ~15%
	ECQuartile = qrcode.High    // ~25%
	ECHigh     = qrcode.Highest // ~30%, required for logos
)
