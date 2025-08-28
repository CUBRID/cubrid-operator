/*
 * Copyright 2016 CUBRID Corporation
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package pkg

import (
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
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
			"group":       groupName + DEF.SELECTOR_SUFFIX,
			"grouptype":   groupType,
			"servicename": serviceName,
		},
	}
}
