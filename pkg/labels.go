package pkg

import (
	"github.com/cubrid/cubrid-operator/pkg/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewLabels(name string) map[string]string {
	return map[string]string{
		"app": name,
	}
}

func NewLabelSelector(appName, groupName, groupType, serviceName string) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app":         appName,
			"group":       groupName + settings.SELECTOR_SUFFIX, // airnet check
			"grouptype":   groupType,
			"servicename": serviceName,
		},
	}
}
