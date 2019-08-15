/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package kubernetes

import (
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	extensions_v1beta1 "k8s.io/api/extensions/v1beta1"
)

var (
	DeploymentKind = apps_v1.SchemeGroupVersion.WithKind("Deployment")
	ServiceKind = core_v1.SchemeGroupVersion.WithKind("Service")
	IngressKind = extensions_v1beta1.SchemeGroupVersion.WithKind("Ingress")
	NamespaceKind = core_v1.SchemeGroupVersion.WithKind("Namespace")
	PVCKind = core_v1.SchemeGroupVersion.WithKind("PersistentVolumeClaim")
	PodKind = core_v1.SchemeGroupVersion.WithKind("Pod")
	EventKind = core_v1.SchemeGroupVersion.WithKind("Event")
)
