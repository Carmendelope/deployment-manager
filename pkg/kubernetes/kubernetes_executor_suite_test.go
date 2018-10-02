/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package kubernetes

import (
    "testing"
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
)

func TestKubernetesExectorTest(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Kubernetes executor Suite")
}
