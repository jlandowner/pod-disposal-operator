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
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	pdov1 "pod-disposal-operator/api/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNow(t *testing.T) {
	r := &realClock{}
	t.Logf("Now: %s", r.Now().String())
}

func TestIgnoreNotFound(t *testing.T) {
	q := &schema.GroupResource{Group: "testGrp", Resource: "testRes"}
	notFoundErr := apierrs.NewNotFound(*q, "tests")
	otherErr := errors.New("Other Error")

	t.Logf("ignoreNotFound will return nil when err is notFound")
	assert.Nil(t, ignoreNotFound(notFoundErr))

	t.Logf("ignoreNotFound will return direct err when err is NOT notFound")
	assert.EqualError(t, ignoreNotFound(otherErr), otherErr.Error())
}

func TestGetNextSchedule(t *testing.T) {
	st := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	parseFunc := func(s string) time.Duration {
		d, _ := time.ParseDuration(s)
		return d
	}

	tests := []struct {
		cron   string
		expect time.Duration
	}{
		{
			// Everyday AM 1:00
			cron:   "0 1 * * *",
			expect: parseFunc("1h"),
		},
		{
			// Every 10th AM 0:00
			cron:   "0 0 10 * *",
			expect: parseFunc("216h"),
		},
		{
			// Every 15 minutes
			cron:   "*/15 * * * *",
			expect: parseFunc("15m"),
		},
	}
	for _, test := range tests {
		t.Log("cron", test.cron, "expect", test.expect)
		next, err := getNextSchedule(test.cron, st)
		assert.Nil(t, err)
		assert.Equal(t, st.Add(test.expect), next)
	}

	// Cron Parse Err Case
	errCronPattern := "* * * * * *"
	_, err := getNextSchedule(errCronPattern, st)
	assert.NotNil(t, err)
	assert.True(t, func() bool { match, _ := regexp.MatchString("^Unparseable schedule", err.Error()); return match }())
}

func TestIsDefaultTime(t *testing.T) {
	result := getDefaultTime()
	assert.Equal(t, time.Date(2006, 1, 2, 0, 0, 0, 0, time.Local), result)

	assert.True(t, isDefaultTime(result))
	assert.True(t, isDefaultTime(initT))
	assert.False(t, isDefaultTime(result.Add(time.Microsecond)))

	tests := []struct {
		time   time.Time
		expect bool
	}{
		{
			time:   result,
			expect: true,
		},
		{
			time:   initT,
			expect: true,
		},
		{
			time:   result.Add(time.Microsecond),
			expect: false,
		},
	}
	for _, test := range tests {
		t.Log("time", test.time.String(), "expect", test.expect)
		assert.Equal(t, test.expect, isDefaultTime(test.time))
	}
}

func TestIsTimingToDisposal(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		time   time.Time
		expect bool
	}{
		{
			// Test: Next is default time
			time:   getDefaultTime(),
			expect: false,
		},
		{
			// Test: Next < Now
			time:   time.Date(2019, 12, 31, 23, 59, 59, 99, time.UTC),
			expect: true,
		},
		{
			// Test: Next > Now
			time:   time.Date(2020, 1, 1, 0, 0, 0, 1, time.UTC),
			expect: false,
		},
		{
			// Test: Next = Now
			time:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			expect: false,
		},
	}

	for _, test := range tests {
		t.Log("time", test.time.String(), "expect", test.expect)
		assert.Equal(t, test.expect, isTimingToDisposal(test.time, now))
	}
}

func TestIsLivingEnough(t *testing.T) {
	lifespan, err := time.ParseDuration("1h")
	assert.Nil(t, err)
	birth := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		now    time.Time
		expect bool
	}{
		{
			// Test: Now - Birth > lifespan
			now:    time.Date(2020, 1, 1, 2, 0, 0, 0, time.UTC),
			expect: true,
		},
		{
			// Test: Now - Birth < lifespan
			now:    time.Date(2020, 1, 1, 0, 59, 59, 0, time.UTC),
			expect: false,
		},
		{
			// Test: Now - Birth = lifespan
			now:    time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
			expect: false,
		},
	}

	for _, test := range tests {
		t.Log("now", test.now, "expect", test.expect)
		assert.Equal(t, test.expect, isLivingEnough(lifespan, birth, test.now))
	}
}

func TestIsRunning(t *testing.T) {
	tests := []struct {
		pod    corev1.Pod
		expect bool
	}{
		{
			// Test name, num of containers, restarts, container ready status
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test1"},
				Spec:       corev1.PodSpec{Containers: make([]corev1.Container, 2)},
				Status: corev1.PodStatus{
					Phase: "podPhase",
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, RestartCount: 3, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
						{RestartCount: 3},
					},
				},
			},
			expect: false,
		},
		{
			// Test container error overwrites pod phase
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test2"},
				Spec:       corev1.PodSpec{Containers: make([]corev1.Container, 2)},
				Status: corev1.PodStatus{
					Phase: "podPhase",
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, RestartCount: 3, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
						{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerWaitingReason"}}, RestartCount: 3},
					},
				},
			},
			expect: false,
		},
		{
			// Test the same as the above but with Terminated state and the first container overwrites the rest
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test3"},
				Spec:       corev1.PodSpec{Containers: make([]corev1.Container, 2)},
				Status: corev1.PodStatus{
					Phase: "podPhase",
					ContainerStatuses: []corev1.ContainerStatus{
						{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerWaitingReason"}}, RestartCount: 3},
						{State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "ContainerTerminatedReason"}}, RestartCount: 3},
					},
				},
			},
			expect: false,
		},
		{
			// Test ready is not enough for reporting running
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test4"},
				Spec:       corev1.PodSpec{Containers: make([]corev1.Container, 2)},
				Status: corev1.PodStatus{
					Phase: "podPhase",
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, RestartCount: 3, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
						{Ready: true, RestartCount: 3},
					},
				},
			},
			expect: false,
		},
		{
			// Test ready is not enough for reporting running
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test5"},
				Spec:       corev1.PodSpec{Containers: make([]corev1.Container, 2)},
				Status: corev1.PodStatus{
					Reason: "podReason",
					Phase:  "podPhase",
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, RestartCount: 3, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
						{Ready: true, RestartCount: 3},
					},
				},
			},
			expect: false,
		},
		{
			// Test pod has 2 containers, one is running and the other is completed, w/o ready condition
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test6"},
				Spec:       corev1.PodSpec{Containers: make([]corev1.Container, 2)},
				Status: corev1.PodStatus{
					Phase:  "Running",
					Reason: "",
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, RestartCount: 3, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Completed", ExitCode: 0}}},
						{Ready: true, RestartCount: 3, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
					},
				},
			},
			expect: true,
		},
		{
			// Test pod has 2 containers and init container, one is running and the other is completed, with ready condition
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test7"},
				Spec:       corev1.PodSpec{Containers: make([]corev1.Container, 2)},
				Status: corev1.PodStatus{
					Phase:  "Running",
					Reason: "",
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, RestartCount: 3, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Completed", ExitCode: 0}}},
						{Ready: true, RestartCount: 3, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
					},
					Conditions: []corev1.PodCondition{
						{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastProbeTime: metav1.Time{Time: time.Now()}},
					},
					InitContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, RestartCount: 3, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Completed", ExitCode: 0}}},
					},
				},
			},
			expect: true,
		},
		{
			// Test init container is running
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test8"},
				Spec:       corev1.PodSpec{Containers: make([]corev1.Container, 1)},
				Status: corev1.PodStatus{
					Phase:  "Running",
					Reason: "",
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, RestartCount: 3, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
					},
					Conditions: []corev1.PodCondition{
						{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastProbeTime: metav1.Time{Time: time.Now()}},
					},
					InitContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, RestartCount: 3, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
					},
				},
			},
			expect: false,
		},
		{
			// Test phase is Completed but container is running
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test9"},
				Spec:       corev1.PodSpec{Containers: make([]corev1.Container, 1)},
				Status: corev1.PodStatus{
					Phase:  "Completed",
					Reason: "",
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, RestartCount: 3, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
					},
					Conditions: []corev1.PodCondition{
						{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastProbeTime: metav1.Time{Time: time.Now()}},
					},
				},
			},
			expect: true,
		},
		{
			// Test phase is Completed but container is running
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test10"},
				Spec:       corev1.PodSpec{Containers: make([]corev1.Container, 1)},
				Status: corev1.PodStatus{
					Phase:  "Running",
					Reason: "",
				},
			},
			expect: true,
		},
	}
	for _, test := range tests {
		t.Log("pod", test.pod.Name, "expect", test.expect)
		assert.Equal(t, test.expect, isRunning(&test.pod))
	}
}

func TestSlicePodsByNumber(t *testing.T) {
	pods := &corev1.PodList{Items: make([]corev1.Pod, 3)}
	for i := range pods.Items {
		pods.Items[i].ObjectMeta.Name = fmt.Sprintf("pod-%d", i)
		pods.Items[i].ObjectMeta.CreationTimestamp = metav1.NewTime(time.Date(2020, 1, 1, i, 0, 0, 0, time.UTC))
	}

	testNumberOfPods := []int{0, 1, 2, 3, 4, 5}
	for _, testNum := range testNumberOfPods {
		podSlice := slicePodsByNumber(pods, testNum)
		t.Log("testNum", testNum, len(podSlice.Items))

		if testNum <= len(pods.Items) {
			assert.Equal(t, testNum, len(podSlice.Items))
		} else {
			assert.Equal(t, len(pods.Items), len(podSlice.Items))
		}

		for i := 0; i < len(podSlice.Items); i++ {
			assert.Equal(t, fmt.Sprintf("pod-%d", i), podSlice.Items[i].Name)
		}
	}
}

func TestGetEffectiveDisposalConcurrency(t *testing.T) {
	tests := []struct {
		minAvailable        int
		disposalConcurrency int
		numberOfPods        int
		expect              int
	}{
		{
			minAvailable:        0,
			disposalConcurrency: 2,
			numberOfPods:        3,
			expect:              2,
		},
		{
			minAvailable:        1,
			disposalConcurrency: 2,
			numberOfPods:        3,
			expect:              2,
		},
		{
			minAvailable:        2,
			disposalConcurrency: 2,
			numberOfPods:        3,
			expect:              1,
		},
		{
			minAvailable:        3,
			disposalConcurrency: 2,
			numberOfPods:        3,
			expect:              0,
		},

		{
			minAvailable:        4,
			disposalConcurrency: 2,
			numberOfPods:        3,
			expect:              0,
		},
	}

	for _, test := range tests {
		pds := pdov1.PodDisposalSchedule{}
		pds.Spec.Strategy.MinAvailable = test.minAvailable
		pds.Spec.Strategy.DisposalConcurrency = test.disposalConcurrency

		t.Log("minAvailable", test.minAvailable, "disposalConcurrency", test.disposalConcurrency, "numberOfPods", test.numberOfPods, "expect", test.expect)
		assert.Equal(t, test.expect, getEffectiveDisposalConcurrency(pds, test.numberOfPods))
	}
}
