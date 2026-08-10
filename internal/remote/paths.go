package remote

import "patty/internal/config"

// defaultManagedKnownHosts is the patty-managed known_hosts path. It is a
// thin indirection over config so tests can leave HostKeyPolicy.ManagedPath
// empty and still get an isolated file under PATTY_HOME.
func defaultManagedKnownHosts() string {
	return config.RemoteKnownHostsPath()
}
