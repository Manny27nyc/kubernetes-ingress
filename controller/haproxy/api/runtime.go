package api

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

func (c *clientNative) ExecuteRaw(command string) (result []string, err error) {
	return c.nativeAPI.Runtime.ExecuteRaw(command)
}

func (c *clientNative) SetServerAddr(backendName string, serverName string, ip string, port int) error {
	return c.nativeAPI.Runtime.SetServerAddr(backendName, serverName, ip, port)
}

func (c *clientNative) SetServerState(backendName string, serverName string, state string) error {
	return c.nativeAPI.Runtime.SetServerState(backendName, serverName, state)
}

func (c *clientNative) SetMapContent(mapFile string, payload string) error {
	err := c.nativeAPI.Runtime.ClearMap(mapFile, false)
	if err != nil {
		return err
	}
	return c.nativeAPI.Runtime.AddMapPayload(mapFile, payload)
}

func (c *clientNative) GetMap(mapFile string) (*models.Map, error) {
	return c.nativeAPI.Runtime.GetMap(mapFile)
}

// SyncBackendSrvs syncs states and addresses of a backend servers with corresponding endpoints.
func (c *clientNative) SyncBackendSrvs(backend *store.RuntimeBackend, portUpdated bool) error {
	if backend.Name == "" {
		return nil
	}
	haproxySrvs := backend.HAProxySrvs
	addresses := backend.Endpoints.Addresses
	// Disable stale entries from HAProxySrvs
	// and provide list of Disabled Srvs
	var disabled []*store.HAProxySrv
	var errors utils.Errors
	for i, srv := range haproxySrvs {
		srv.Modified = srv.Modified || portUpdated
		if _, ok := addresses[srv.Address]; ok {
			delete(addresses, srv.Address)
		} else {
			haproxySrvs[i].Address = ""
			haproxySrvs[i].Modified = true
			disabled = append(disabled, srv)
		}
	}

	// Configure new Addresses in available HAProxySrvs
	for newAddr := range addresses {
		if len(disabled) == 0 {
			break
		}
		disabled[0].Address = newAddr
		disabled[0].Modified = true
		disabled = disabled[1:]
		delete(addresses, newAddr)
	}
	// Dynamically updates HAProxy backend servers  with HAProxySrvs content
	var addrErr, stateErr error
	for _, srv := range haproxySrvs {
		if !srv.Modified {
			continue
		}
		if srv.Address == "" {
			// logger.Tracef("server '%s/%s' changed status to %v", newEndpoints.BackendName, srv.Name, "maint")
			addrErr = c.SetServerAddr(backend.Name, srv.Name, "127.0.0.1", 0)
			stateErr = c.SetServerState(backend.Name, srv.Name, "maint")
		} else {
			// logger.Tracef("server '%s/%s' changed status to %v", newEndpoints.BackendName, srv.Name, "ready")
			addrErr = c.SetServerAddr(backend.Name, srv.Name, srv.Address, int(backend.Endpoints.Port))
			stateErr = c.SetServerState(backend.Name, srv.Name, "ready")
		}
		if addrErr != nil || stateErr != nil {
			backend.DynUpdateFailed = true
			errors.Add(addrErr)
			errors.Add(stateErr)
		}
	}
	return errors.Result()
}
