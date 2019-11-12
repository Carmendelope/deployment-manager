/*
 * Copyright 2019 Nalej
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package kubernetes

import (
	"flag"
	"github.com/nalej/derrors"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"regexp"
)

// Collection of common entries regarding Kubernetes internal operations.

const (
	// Regular expression to be followed by labels
	KUBERNETES_LABEL_EXPR = "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
	// Regular expression to find invalid label characters
	KUBERNETES_LABEL_INVALID_EXPR = "[^-a-z0-9]"
)

// Regular expression checker to validate labels
var kubernetesLabelsChecker = regexp.MustCompile(KUBERNETES_LABEL_EXPR)

// Regular expression to find invalid character labels
var kubernetesInvalidLabelChar = regexp.MustCompile(KUBERNETES_LABEL_INVALID_EXPR)

// Transform an incoming string to a K8s compatible format
// params:
//  input
// return:
//  label adapted to k8s
func ReformatLabel(input string) string {
	_, fullString := kubernetesLabelsChecker.LiteralPrefix()
	if fullString {
		return input
	}
	adapted := kubernetesInvalidLabelChar.ReplaceAllString(input, "")
	return adapted
}

// Create a Kubernetes Client to interact as kubectl
func GetKubernetesClient(internal bool) (*kubernetes.Clientset, derrors.Error) {
	var c *kubernetes.Clientset
	var err error

	if internal {
		c, err = getInternalKubernetesClient()
	} else {
		c, err = getExternalKubernetesClient()
	}
	if err != nil {
		log.Error().Err(err).Msg("impossible to create kubernetes clientset")
		return nil, derrors.AsError(err, "impossible to create kubernetes Client")
	}
	if c == nil {
		log.Error().Msg("kubernetes Client set is nil")
		return nil, derrors.NewInternalError("kubernetes Client set was nil")
	}
	return c, nil
}

// Create a new kubernetes Client using deployment inside the cluster.
//  params:
//   internal true if the Client is deployed inside the cluster.
//  return:
//   instance for the k8s Client or error if any
func getInternalKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Panic().Err(err).Msg("impossible to get local configuration for internal k8s Client")
		return nil, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panic().Err(err).Msg("impossible to instantiate k8s Client")
		return nil, err
	}
	return clientset, nil
}

// Figure out the filename of the Kubernetes configuration
func KubeConfigPath() string {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	return *kubeconfig
}

// Create a new kubernetes Client using deployment outside the cluster.
//  params:
//   internal true if the Client is deployed inside the cluster.
//  return:
//   instance for the k8s Client or error if any
func getExternalKubernetesClient() (*kubernetes.Clientset, error) {
	kubeconfig := KubeConfigPath()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Panic().Err(err).Msg("error building configuration from kubeconfig")
		return nil, err
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panic().Err(err).Msg("error using configuration to build k8s clientset")
		return nil, err
	}

	return clientset, nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// Kubernetes API requires pointers to simple types. These are some functions to help with this.
func getBool(value bool) *bool {
	aux := value
	return &aux
}
