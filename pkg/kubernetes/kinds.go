/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 *
 *
 */

package kubernetes

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
)

var (
	DeploymentKind = appsv1.SchemeGroupVersion.WithKind("Deployment")
	ServiceKind = corev1.SchemeGroupVersion.WithKind("Service")
	IngressKind = extensionsv1beta1.SchemeGroupVersion.WithKind("Ingress")
	NamespaceKind = corev1.SchemeGroupVersion.WithKind("Namespace")
	PVCKind = corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim")
	PodKind = corev1.SchemeGroupVersion.WithKind("Pod")
	EventKind = corev1.SchemeGroupVersion.WithKind("Event")
)
