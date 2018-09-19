/*
 * Copyright 2018 Nalej
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
    "os"
    "path/filepath"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    apiv1 "k8s.io/api/core/v1"
    appsv1 "k8s.io/api/apps/v1"
    // Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
    // _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
    "k8s.io/client-go/rest"
    "github.com/rs/zerolog/log"
    "time"
)

type KubernetesClient struct {
    client *kubernetes.Clientset
}

func NewKubernetesClient(internal bool) (*KubernetesClient,error) {
    var client *kubernetes.Clientset
    var err error
    if internal {
        client, err = getInternalKubernetesClient()
    } else {
        client, err = getExternalKubernetesClient()
    }
    toReturn := KubernetesClient{client}
    return &toReturn, err
}

func (c *KubernetesClient) Test() {
    for {
        pods, err := c.client.CoreV1().Pods("").List(metav1.ListOptions{})
        if err != nil {
            panic(err.Error())
        }
        log.Info().Msgf("There are %d pods in the cluster\n", len(pods.Items))
        time.Sleep(10 * time.Second)
    }
}

func (c *KubernetesClient) RunDeployment() {
    deployment := &appsv1.Deployment{
        ObjectMeta: metav1.ObjectMeta{
            Name: "demo-deployment",
        },
        Spec: appsv1.DeploymentSpec{
            Replicas: int32Ptr(2),
            Selector: &metav1.LabelSelector{
                MatchLabels: map[string]string{
                    "app": "deployment-manager",
                },
            },
            Template: apiv1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Labels: map[string]string{
                        "app": "deployment-manager",
                    },
                },
                Spec: apiv1.PodSpec{
                    Containers: []apiv1.Container{
                        {
                            Name:  "web",
                            Image: "nginx:1.12",
                            Ports: []apiv1.ContainerPort{
                                {
                                    Name:          "http",
                                    Protocol:      apiv1.ProtocolTCP,
                                    ContainerPort: 80,
                                },
                            },
                        },
                    },
                },
            },
        },
    }

    deploymentsClient := c.client.AppsV1().Deployments(apiv1.NamespaceDefault)
    result, err := deploymentsClient.Create(deployment)
    if err != nil {
        panic(err)
    }
    log.Info().Interface("result",result).Msg("---->")
}

func int32Ptr(i int32) *int32 { return &i }

// Create a new kubernetes client using deployment inside the cluster.
//  params:
//   internal true if the client is deployed inside the cluster.
//  return:
//   instance for the k8s client or error if any
func getInternalKubernetesClient() (*kubernetes.Clientset,error) {
    config, err := rest.InClusterConfig()
    if err != nil {
        log.Panic().Err(err).Msg("impossible to get local configuration for internal k8s client")
        return nil, err
    }
    // creates the clientset
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        log.Panic().Err(err).Msg("impossible to instantiate k8s client")
        return nil, err
    }
    return clientset,nil
}


// Create a new kubernetes client using deployment outside the cluster.
//  params:
//   internal true if the client is deployed inside the cluster.
//  return:
//   instance for the k8s client or error if any
func getExternalKubernetesClient() (*kubernetes.Clientset,error) {
    var kubeconfig *string
    if home := homeDir(); home != "" {
        kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
    } else {
        kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
    }
    flag.Parse()

    // use the current context in kubeconfig
    config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
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


