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
	"fmt"
	pdov1 "pod-disposal-operator/api/v1"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSortPodsByOrder(t *testing.T) {
	pods := &corev1.PodList{Items: make([]corev1.Pod, 3)}
	for i := 0; i < 3; i++ {
		pods.Items[i].ObjectMeta.Name = fmt.Sprintf("pod-%d", i)
		pods.Items[i].ObjectMeta.CreationTimestamp = metav1.NewTime(time.Date(2020, 1, 1, i, 0, 0, 0, time.UTC))
	}
	assert.Equal(t, 3, len(pods.Items))

	// Old order test
	oldOrderTests := []struct {
		podlist *corev1.PodList
		order   pdov1.OrderType
	}{
		{
			podlist: &corev1.PodList{
				Items: []corev1.Pod{pods.Items[1], pods.Items[0], pods.Items[2]},
			},
			order: pdov1.OldOrder,
		},
		{
			podlist: &corev1.PodList{
				Items: []corev1.Pod{pods.Items[1], pods.Items[2], pods.Items[0]},
			},
			order: pdov1.OldOrder,
		},
	}
	for _, test := range oldOrderTests {
		newPods := sortPodsByOrder(test.podlist, test.order)
		for i := 0; i < 3; i++ {
			assert.Equal(t, pods.Items[i].Name, newPods.Items[i].Name)
			assert.Equal(t, pods.Items[i].CreationTimestamp.Time, newPods.Items[i].CreationTimestamp.Time)
		}
	}
}
