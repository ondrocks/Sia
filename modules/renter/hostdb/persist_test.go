package hostdb

import (
	// "crypto/rand"
	"path/filepath"
	"testing"

	// "github.com/NebulousLabs/Sia/build"
	"github.com/NebulousLabs/Sia/modules"
	// "github.com/NebulousLabs/Sia/modules/gateway"
	// "github.com/NebulousLabs/Sia/types"
)

// quitAfterLoadDeps will quit startup in newHostDB
type quitAfterLoadDeps struct {
	prodDependencies
}

// Send a disrupt signal to the quitAfterLoad codebreak.
func (quitAfterLoadDeps) disrupt(s string) bool {
	if s == "quitAfterLoad" {
		return true
	}
	return false
}

// TestSaveLoad tests that the hostdb can save and load itself.
//
// TODO: By extending the hdbTester and adding some helper functions, we can
// eliminate the necessary disruption by adding real hosts + blocks instead of
// fake ones.
func TestSaveLoad(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()
	hdbt, err := newHDBTester("TestSaveLoad")
	if err != nil {
		t.Fatal(err)
	}

	// Add fake hosts and a fake consensus change. The fake consensus change
	// would normally be detected and routed around, but we stunt the loading
	// process to only load the persistent fields.
	var host1, host2, host3 modules.HostDBEntry
	host1.PublicKey.Key = []byte("foo")
	host2.PublicKey.Key = []byte("bar")
	host3.PublicKey.Key = []byte("baz")
	hdbt.hdb.hostTree.Insert(host1)
	hdbt.hdb.hostTree.Insert(host2)
	hdbt.hdb.hostTree.Insert(host3)
	hdbt.hdb.lastChange = modules.ConsensusChangeID{1, 2, 3}
	stashedLC := hdbt.hdb.lastChange

	// Save, close, and reload.
	err = hdbt.hdb.save()
	if err != nil {
		t.Fatal(err)
	}
	err = hdbt.hdb.Close()
	if err != nil {
		t.Fatal(err)
	}
	hdbt.hdb, err = newHostDB(hdbt.gateway, hdbt.cs, filepath.Join(hdbt.persistDir, modules.RenterDir), quitAfterLoadDeps{})
	if err != nil {
		t.Fatal(err)
	}

	// Last change should have been reloaded.
	if hdbt.hdb.lastChange != stashedLC {
		t.Error("wrong consensus change ID was loaded:", hdbt.hdb.lastChange)
	}

	// Check that AllHosts was loaded.
	_, ok0 := hdbt.hdb.hostTree.Select(host1.PublicKey)
	_, ok1 := hdbt.hdb.hostTree.Select(host2.PublicKey)
	_, ok2 := hdbt.hdb.hostTree.Select(host3.PublicKey)
	if !ok0 || !ok1 || !ok2 || len(hdbt.hdb.hostTree.All()) != 3 {
		t.Error("allHosts was not restored properly", ok0, ok1, ok2, len(hdbt.hdb.hostTree.All()))
	}
}

// TestRescan tests that the hostdb will rescan the blockchain properly, picking
// up new hosts which appear in an alternate past.
func TestRescan(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	_, err := newHDBTester("TestRescan")
	if err != nil {
		t.Fatal(err)
	}

	t.Skip("create two consensus sets with blocks + announcements")
}
