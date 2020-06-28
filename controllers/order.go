/*
Copyright 2020 jlandowner.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	pdov1 "pod-disposal-operator/api/v1"
	"sort"

	corev1 "k8s.io/api/core/v1"
)

// sortPodsByOrder returns sorted podlist in order of given strategy.
func sortPodsByOrder(pods *corev1.PodList, order pdov1.OrderType) *corev1.PodList {
	switch order {
	case pdov1.OldOrder:
		return sortOldOrder(pods)
	default:
		return pods
	}
}

// sortOldOrder returns new podlist sorted by old order.
func sortOldOrder(pods *corev1.PodList) *corev1.PodList {
	newPods := pods.DeepCopy()
	sort.Slice(newPods.Items, func(i, j int) bool {
		return newPods.Items[i].CreationTimestamp.Before(&newPods.Items[j].CreationTimestamp)
	})
	return newPods
}
