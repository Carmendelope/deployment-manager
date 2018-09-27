/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package handler

import (
    "testing"
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
)

func TestKubernetesExectorTest(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Deploy manager handler test suite")
}
