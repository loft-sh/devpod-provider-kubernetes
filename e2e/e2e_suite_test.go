package e2e

import (
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	// Register tests
	_ "github.com/loft-sh/devpod-provider-kubernetes/e2e/pullsecrets"
)

func TestRunE2ETests(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Devpod provider kubernetes e2e suite")
}
