/*
 *  Copyright (C) 2018 Nalej Group - All Rights Reserved
 */


package kubernetes

import (
    "github.com/rs/zerolog/log"
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
        log.Debug().Msg("label matches k8s definition")
        return input
    }
    adapted := kubernetesInvalidLabelChar.ReplaceAllString(input,"")
    return adapted
}