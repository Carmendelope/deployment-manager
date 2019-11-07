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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
)

var (
	DeploymentKind = appsv1.SchemeGroupVersion.WithKind("Deployment")
	ServiceKind    = corev1.SchemeGroupVersion.WithKind("Service")
	IngressKind    = extensionsv1beta1.SchemeGroupVersion.WithKind("Ingress")
	NamespaceKind  = corev1.SchemeGroupVersion.WithKind("Namespace")
	PVCKind        = corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim")
	PodKind        = corev1.SchemeGroupVersion.WithKind("Pod")
	EventKind      = corev1.SchemeGroupVersion.WithKind("Event")
)
