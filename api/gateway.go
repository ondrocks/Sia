package api

import (
	"net/http"

	"github.com/NebulousLabs/Sia/modules"
)

type GatewayInfo struct {
	Address modules.NetAddress
	Peers   []modules.NetAddress
}

// gatewayStatusHandler handles the API call asking for the gatway status.
func (srv *Server) gatewayStatusHandler(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, GatewayInfo{srv.gateway.Address(), srv.gateway.Peers()})
}

// gatewayPeerAddHandler handles the API call to add a peer to the gateway.
func (srv *Server) gatewayPeerAddHandler(w http.ResponseWriter, req *http.Request) {
	addr := modules.NetAddress(req.FormValue("address"))
	err := srv.gateway.Connect(addr)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeSuccess(w)
}

// gatewayPeerRemoveHandler handles the API call to remove a peer from the gateway.
func (srv *Server) gatewayPeerRemoveHandler(w http.ResponseWriter, req *http.Request) {
	addr := modules.NetAddress(req.FormValue("address"))
	err := srv.gateway.Disconnect(addr)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeSuccess(w)
}