package kubernetes

type dryRunConfig struct {
	strategy  dryRunStrategy
	manifests []string
}

// dryRunStrategy mimics "k8s.io/kubectl/pkg/cmd/util".DryRun but as a string so it can be passed to kubectl
type dryRunStrategy string

const (
	// DryRunNone indicates the client will make all mutating calls
	DryRunNone dryRunStrategy = ""

	// DryRunClient, or client-side dry-run, indicates the client will prevent
	// making mutating calls such as CREATE, PATCH, and DELETE
	DryRunClient = "client"

	// DryRunServer, or server-side dry-run, indicates the client will send
	// mutating calls to the APIServer with the dry-run parameter to prevent
	// persisting changes.
	//
	// Note that clients sending server-side dry-run calls should verify that
	// the APIServer and the resource supports server-side dry-run, and otherwise
	// clients should fail early.
	//
	// If a client sends a server-side dry-run call to an APIServer that doesn't
	// support server-side dry-run, then the APIServer will persist changes inadvertently.
	DryRunServer = "server"
)

func NewDryRunConfig(strategy dryRunStrategy) *dryRunConfig {
	return &dryRunConfig{
		strategy:  strategy,
		manifests: []string{},
	}
}

func (d *dryRunConfig) AddManifest(rawManifest []byte) {
	d.manifests = append(d.manifests, string(rawManifest))
}
