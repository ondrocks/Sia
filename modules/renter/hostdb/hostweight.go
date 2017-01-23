package hostdb

import (
	"math/big"

	"github.com/NebulousLabs/Sia/build"
	"github.com/NebulousLabs/Sia/modules"
	"github.com/NebulousLabs/Sia/types"
)

var (
	// Because most weights would otherwise be fractional, we set the base
	// weight to be very large.
	baseWeight = types.NewCurrency(new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil))

	// collateralExponentiation is the number of times that the collateral is
	// multiplied into the price.
	//
	// NOTE: Changing this value downwards needs that the baseWeight will need
	// to be increased.
	collateralExponentiation = 2

	// priceDiveNormalization reduces the raw value of the price so that not so
	// many digits are needed when operating on the weight. This also allows the
	// base weight to be a lot lower.
	priceDivNormalization = types.SiacoinPrecision.Div64(100)

	// Set a mimimum price, below which setting lower prices will no longer put
	// this host at an advatnage. This price is considered the bar for
	// 'essentially free', and is kept to a minimum to prevent certain Sybil
	// attack related attack vectors.
	//
	// NOTE: This needs to be intelligently adjusted down as the practical price
	// of storage changes, and as the price of the siacoin changes.
	minDivPrice = types.SiacoinPrecision.Mul64(250)

	// priceExponentiation is the number of times that the weight is divided by
	// the price.
	//
	// NOTE: Changing this value upwards means that the baseWeight will need to
	// be increased.
	priceExponentiation = 4

	// requiredStorage indicates the amount of storage that the host must be
	// offering in order to be considered a valuable/worthwhile host.
	requiredStorage = func() uint64 {
		switch build.Release {
		case "dev":
			return 1e6
		case "standard":
			return 5e9
		case "testing":
			return 1e3
		default:
			panic("incorrect/missing value for requiredStorage constant")
		}
	}()
)

// collateralAdjustments improves the host's weight according to the amount of
// collateral that they have provided.
//
// NOTE: For any reasonable value of collateral, there will be a huge blowup,
// allowing for the base weight to be a lot lower, as the collateral is
// accounted for before anything else.
func collateralAdjustments(entry modules.HostDBEntry, weight types.Currency) types.Currency {
	if entry.Collateral.IsZero() {
		// Instead of zeroing out the weight, just return the weight as though
		// the collateral is 1 hasting. Competitively speaking, this is
		// effectively zero.
		return weight
	}
	for i := 0; i < collateralExponentiation; i++ {
		weight = weight.Mul(entry.Collateral)
	}
	return weight
}

// priceAdjustments will adjust the weight of the entry according to the prices
// that it has set.
func priceAdjustments(entry modules.HostDBEntry, weight types.Currency) types.Currency {
	// Sanity checks - the constants values need to have certain relationships
	// to eachother
	if build.DEBUG {
		// If the minDivPrice is not much larger than the divNormalization,
		// there will be problems with granularity after the divNormalization is
		// applied.
		if minDivPrice.Div64(100).Cmp(priceDivNormalization) < 0 {
			build.Critical("Maladjusted minDivePrice and divNormalization constants in hostdb package")
		}
	}

	// Prices tiered as follows:
	//    - the storage price is presented as 'per block per byte'
	//    - the contract price is presented as a flat rate
	//    - the upload bandwidth price is per byte
	//    - the download bandwidth price is per byte
	//
	// The hostdb will naively assume the following for now:
	//    - each contract covers 6 weeks of storage (default is 12 weeks, but
	//      renewals occur at midpoint) - 6048 blocks - and 10GB of storage.
	//    - uploads happen once per 12 weeks (average lifetime of a file is 12 weeks)
	//    - downloads happen once per 6 weeks (files are on average downloaded twice throughout lifetime)
	//
	// In the future, the renter should be able to track average user behavior
	// and adjust accordingly. This flexibility will be added later.
	adjustedContractPrice := entry.ContractPrice.Div64(6048).Div64(10e9) // Adjust contract price to match 10GB for 6 weeks.
	adjustedUploadPrice := entry.UploadBandwidthPrice.Div64(24192)       // Adjust upload price to match a single upload over 24 weeks.
	adjustedDownloadPrice := entry.DownloadBandwidthPrice.Div64(12096)   // Adjust download price to match one download over 12 weeks.
	siafundFee := adjustedContractPrice.Add(adjustedUploadPrice).Add(adjustedDownloadPrice).Add(entry.Collateral).MulTax()
	totalPrice := entry.StoragePrice.Add(adjustedContractPrice).Add(adjustedUploadPrice).Add(adjustedDownloadPrice).Add(siafundFee)

	// Set the divPrice, which is closely related to the totalPrice, but
	// adjusted both to make the math more computationally friendly and also
	// given a hard minimum to prevent certain classes of Sybil attacks -
	// attacks where the attacker tries to esacpe the need to burn coins by
	// setting an extremely low price.
	divPrice := totalPrice
	if divPrice.Cmp(minDivPrice) < 0 {
		divPrice = minDivPrice
	}
	// Shrink the div price so that the math can be a lot less intense. Without
	// this step, the base price would need to be closer to 10e150 as opposed to
	// 10e50.
	divPrice = divPrice.Div(priceDivNormalization)
	for i := 0; i < priceExponentiation; i++ {
		weight = weight.Div(divPrice)
	}
	return weight
}

// storageRemainingAdjustments adjusts the weight of the entry according to how
// much storage it has remaining.
func storageRemainingAdjustments(entry modules.HostDBEntry, weight types.Currency) types.Currency {
	if entry.RemainingStorage < 200*requiredStorage {
		weight = weight.Div64(2) // 2x total penalty
	}
	if entry.RemainingStorage < 100*requiredStorage {
		weight = weight.Div64(3) // 6x total penalty
	}
	if entry.RemainingStorage < 50*requiredStorage {
		weight = weight.Div64(4) // 24x total penalty
	}
	if entry.RemainingStorage < 25*requiredStorage {
		weight = weight.Div64(5) // 95x total penalty
	}
	if entry.RemainingStorage < 10*requiredStorage {
		weight = weight.Div64(6) // 570x total penalty
	}
	if entry.RemainingStorage < 5*requiredStorage {
		weight = weight.Div64(10) // 5,700x total penalty
	}
	if entry.RemainingStorage < requiredStorage {
		weight = weight.Div64(100) // 570,000x total penalty
	}
	return weight
}

// versionAdjustments will adjust the weight of the entry according to the siad
// version reported by the host.
func versionAdjustments(entry modules.HostDBEntry, weight types.Currency) types.Currency {
	if build.VersionCmp(entry.Version, "1.0.3") < 0 {
		weight = weight.Div64(5) // 5x total penalty.
	}
	if build.VersionCmp(entry.Version, "1.0.0") < 0 {
		weight = weight.Div64(20) // 100x total penalty.
	}
	return weight
}

// lifetimeAdjustments will adjust the weight of the host according to the total
// amount of time that has passed since the host's original announcement.
func (hdb *HostDB) lifetimeAdjustments(entry modules.HostDBEntry, weight types.Currency) types.Currency {
	if hdb.blockHeight >= entry.FirstSeen {
		age := hdb.blockHeight - entry.FirstSeen
		if age < 6000 {
			weight = weight.Div64(2) // 2x total
		}
		if age < 4000 {
			weight = weight.Div64(2) // 4x total
		}
		if age < 2000 {
			weight = weight.Div64(4) // 16x total
		}
		if age < 1000 {
			weight = weight.Div64(4) // 64x total
		}
		if age < 288 {
			weight = weight.Div64(10) // 640x total
		}
	} else {
		// Shouldn't happen, but the usecase is covered anyway.
		weight = weight.Div64(1000) // Because something weird is happening, don't trust this host very much.
		hdb.log.Critical("Hostdb has witnessed a host where the FirstSeen height is higher than the current block height.")
	}
	return weight
}

// calculateHostWeight returns the weight of a host according to the settings of
// the host database entry. Currently, only the price is considered.
func (hdb *HostDB) calculateHostWeight(entry modules.HostDBEntry) types.Currency {
	weight := baseWeight
	weight = collateralAdjustments(entry, weight)
	weight = priceAdjustments(entry, weight)
	weight = storageRemainingAdjustments(entry, weight)
	weight = versionAdjustments(entry, weight)
	weight = hdb.lifetimeAdjustments(entry, weight)

	// A weight of zero is problematic for for the host tree.
	if weight.IsZero() {
		return types.NewCurrency64(1)
	}
	return weight
}
