package types

import (
	"k8s.io/client-go/kubernetes"
)

// RunOptions contains benchmark running options
type RunOptions struct {
	Tenant          string
	TenantNamespace string
	Label           string
	KClient         *kubernetes.Clientset
	TClient         *kubernetes.Clientset
}
