package wallet

import (
	"testing"
	"time"

	"github.com/NebulousLabs/Sia/types"
)

func TestDefragWallet(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	wt, err := createWalletTester("TestDefragWallet")
	if err != nil {
		t.Fatal(err)
	}
	defer wt.closeWt()

	// mine defragThreshold blocks, resulting in defragThreshold outputs
	for i := 0; i < defragThreshold; i++ {
		_, err := wt.miner.AddBlock()
		if err != nil {
			t.Fatal(err)
		}
	}

	// add another block to push the number of outputs over the threshold
	_, err = wt.miner.AddBlock()
	if err != nil {
		t.Fatal(err)
	}

	// allow some time for the defrag transaction to occur, then mine another block
	time.Sleep(time.Second)

	_, err = wt.miner.AddBlock()
	if err != nil {
		t.Fatal(err)
	}

	// defrag should keep the outputs below the threshold
	if len(wt.wallet.siacoinOutputs) > defragThreshold {
		t.Fatalf("defrag should result in fewer than defragThreshold outputs, got %v wanted %v\n", len(wt.wallet.siacoinOutputs), defragThreshold)
	}
}

func TestDefragWalletDust(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	wt, err := createWalletTester("TestDefragWalletDust")
	if err != nil {
		t.Fatal(err)
	}
	defer wt.closeWt()

	dustOutputValue := types.NewCurrency64(10000)
	noutputs := defragThreshold + 1

	tbuilder := wt.wallet.StartTransaction()
	err = tbuilder.FundSiacoins(dustOutputValue.Mul64(uint64(noutputs)))
	if err != nil {
		t.Fatal(err)
	}

	var dest types.UnlockHash
	for k := range wt.wallet.keys {
		dest = k
		break
	}

	for i := 0; i < noutputs; i++ {
		tbuilder.AddSiacoinOutput(types.SiacoinOutput{
			Value:      dustOutputValue,
			UnlockHash: dest,
		})
	}

	txns, err := tbuilder.Sign(true)
	if err != nil {
		t.Fatal(err)
	}

	err = wt.tpool.AcceptTransactionSet(txns)
	if err != nil {
		t.Fatal(err)
	}

	_, err = wt.miner.AddBlock()
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)

	if defragged := wt.wallet.defragWallet(); defragged {
		t.Fatal("defrag consolidated dust outputs")
	}

	if len(wt.wallet.siacoinOutputs) < defragThreshold {
		t.Fatal("defrag consolidated dust outputs")
	}
}