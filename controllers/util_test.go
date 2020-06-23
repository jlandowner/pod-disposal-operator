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

	// Everyday AM 1:00
	everyOneOClockCronPattern := "0 1 * * *"
	expectOneHourLater, _ := time.ParseDuration("1h")

	// Every 10th AM 0:00
	everyTenthOfMonthCronPattern := "0 0 10 * *"
	expectTenDayslater, _ := time.ParseDuration("216h")

	// Every 15 minutes
	everyFifteenMinutesCronPattern := "*/15 * * * *"
	expectFifteenMinuteslater, _ := time.ParseDuration("15m")

	// Cron Parse Err Case
	errCronPattern := "* * * * * *"

	// Everyday AM 1:00
	next, err := getNextSchedule(everyOneOClockCronPattern, st)
	assert.Nil(t, err)
	assert.Equal(t, st.Add(expectOneHourLater), next)

	// Every 10th AM 0:00
	next, err = getNextSchedule(everyTenthOfMonthCronPattern, st)
	assert.Nil(t, err)
	assert.Equal(t, st.Add(expectTenDayslater), next)

	// Every 15 minutes
	next, err = getNextSchedule(everyFifteenMinutesCronPattern, st)
	assert.Nil(t, err)
	assert.Equal(t, st.Add(expectFifteenMinuteslater), next)

	// Cron Parse Err Case
	next, err = getNextSchedule(errCronPattern, st)
	assert.NotNil(t, err)
	assert.True(t, func() bool { match, _ := regexp.MatchString("^Unparseable schedule", err.Error()); return match }())
}

func TestIsDefaultTime(t *testing.T) {
	result := getDefaultTime()
	assert.Equal(t, time.Date(2006, 1, 2, 0, 0, 0, 0, time.Local), result)

	assert.True(t, isDefaultTime(result))
	assert.True(t, isDefaultTime(initT))
	assert.False(t, isDefaultTime(result.Add(time.Microsecond)))
}

func TestIsTimingToDisposal(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	// False: Next is default time
	nextDisposalTime := getDefaultTime()
	assert.False(t, isTimingToDisposal(nextDisposalTime, now))

	// True: Next < Now
	nextDisposalTime = time.Date(2019, 12, 31, 23, 59, 59, 99, time.UTC)
	assert.True(t, isTimingToDisposal(nextDisposalTime, now))

	// False: Next > Now
	nextDisposalTime = time.Date(2020, 1, 1, 0, 0, 0, 1, time.UTC)
	assert.False(t, isTimingToDisposal(nextDisposalTime, now))

	// False: Next = Now
	nextDisposalTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	assert.False(t, isTimingToDisposal(nextDisposalTime, now))
}

func TestIsLivingEnough(t *testing.T) {
	lifespan, err := time.ParseDuration("1h")
	assert.Nil(t, err)
	birth := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	// True: Now - Birth > lifespan
	now := time.Date(2020, 1, 1, 2, 0, 0, 0, time.UTC)
	assert.True(t, isLivingEnough(lifespan, birth, now))

	// False: Now - Birth < lifespan
	now = time.Date(2020, 1, 1, 0, 59, 59, 0, time.UTC)
	assert.False(t, isLivingEnough(lifespan, birth, now))

	// False: Now - Birth = lifespan
	now = time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
	assert.False(t, isLivingEnough(lifespan, birth, now))
}

func TestIsRunning(t *testing.T) {
	tests := []struct {
		pod    corev1.Pod
		expect bool
	}{
		{
			// Test name, num of containers, restarts, container ready status
			corev1.Pod{
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
			false,
		},
		{
			// Test container error overwrites pod phase
			corev1.Pod{
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
			false,
		},
		{
			// Test the same as the above but with Terminated state and the first container overwrites the rest
			corev1.Pod{
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
			false,
		},
		{
			// Test ready is not enough for reporting running
			corev1.Pod{
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
			false,
		},
		{
			// Test ready is not enough for reporting running
			corev1.Pod{
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
			false,
		},
		{
			// Test pod has 2 containers, one is running and the other is completed, w/o ready condition
			corev1.Pod{
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
			true,
		},
		{
			// Test pod has 2 containers and init container, one is running and the other is completed, with ready condition
			corev1.Pod{
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
			true,
		},
		{
			// Test init container is running
			corev1.Pod{
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
			false,
		},
		{
			// Test phase is Completed but container is running
			corev1.Pod{
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
			true,
		},
		{
			// Test phase is Completed but container is running
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test10"},
				Spec:       corev1.PodSpec{Containers: make([]corev1.Container, 1)},
				Status: corev1.PodStatus{
					Phase:  "Running",
					Reason: "",
				},
			},
			true,
		},
	}
	for _, test := range tests {
		assert.Equal(t, test.expect, isRunning(&test.pod))
	}
}

func TestSortPodsByOrder(t *testing.T) {
	pods := make([]corev1.Pod, 3)
	for i := 0; i < 3; i++ {
		pods[i].ObjectMeta.Name = fmt.Sprintf("pod-%d", i)
		pods[i].ObjectMeta.CreationTimestamp = metav1.NewTime(time.Date(2020, 1, 1, i, 0, 0, 0, time.UTC))
	}
	timesortAscTimestampPods := &corev1.PodList{}
	timesortAscTimestampPods.Items = pods
	assert.Equal(t, 3, len(timesortAscTimestampPods.Items))

	randamTimestampPods := &corev1.PodList{}
	randamTimestampPods.Items = append(randamTimestampPods.Items, pods[1])
	randamTimestampPods.Items = append(randamTimestampPods.Items, pods[0])
	randamTimestampPods.Items = append(randamTimestampPods.Items, pods[2])

	randamTimestampPods2 := &corev1.PodList{}
	randamTimestampPods2.Items = append(randamTimestampPods2.Items, pods[1])
	randamTimestampPods2.Items = append(randamTimestampPods2.Items, pods[2])
	randamTimestampPods2.Items = append(randamTimestampPods2.Items, pods[0])

	// OldOrder
	newPods := sortPodsByOrder(randamTimestampPods, pdov1.OldOrder)
	assert.Equal(t, 3, len(newPods.Items))
	for i := 0; i < 3; i++ {
		assert.Equal(t, timesortAscTimestampPods.Items[i].Name, newPods.Items[i].Name)
		assert.Equal(t, timesortAscTimestampPods.Items[i].CreationTimestamp.Time, newPods.Items[i].CreationTimestamp.Time)
	}

	newPods = sortPodsByOrder(randamTimestampPods2, pdov1.OldOrder)
	assert.Equal(t, 3, len(newPods.Items))
	for i := 0; i < 3; i++ {
		assert.Equal(t, timesortAscTimestampPods.Items[i].Name, newPods.Items[i].Name)
		assert.Equal(t, timesortAscTimestampPods.Items[i].CreationTimestamp.Time, newPods.Items[i].CreationTimestamp.Time)
	}
}

func TestSlicePodsByNumber(t *testing.T) {
	pods := make([]corev1.Pod, 3)
	for i := 0; i < 3; i++ {
		pods[i].ObjectMeta.Name = fmt.Sprintf("pod-%d", i)
		pods[i].ObjectMeta.CreationTimestamp = metav1.NewTime(time.Date(2020, 1, 1, i, 0, 0, 0, time.UTC))
	}
	timesortAscTimestampPods := &corev1.PodList{}
	timesortAscTimestampPods.Items = pods

	podSlice := slicePodsByNumber(timesortAscTimestampPods, 0)
	assert.Equal(t, 0, len(podSlice.Items))

	podSlice = slicePodsByNumber(timesortAscTimestampPods, 1)
	assert.Equal(t, 1, len(podSlice.Items))
	assert.Equal(t, "pod-0", podSlice.Items[0].Name)

	podSlice = slicePodsByNumber(timesortAscTimestampPods, 2)
	assert.Equal(t, 2, len(podSlice.Items))
	for _, pod := range podSlice.Items {
		assert.NotEqual(t, "pod-3", pod.Name)
	}

	podSlice = slicePodsByNumber(timesortAscTimestampPods, 3)
	assert.Equal(t, 3, len(podSlice.Items))

	podSlice = slicePodsByNumber(timesortAscTimestampPods, 4)
	assert.Equal(t, 3, len(podSlice.Items))
}

func TestGetEffectiveDisposalConcurrency(t *testing.T) {
	var result int
	pds := pdov1.PodDisposalSchedule{}
	pds.Spec.Strategy.DisposalConcurrency = 2
	numberOfPods := 3

	pds.Spec.Strategy.MinAvailable = 0
	result = getEffectiveDisposalConcurrency(pds, numberOfPods)
	assert.Equal(t, 2, result)

	pds.Spec.Strategy.MinAvailable = 1
	result = getEffectiveDisposalConcurrency(pds, numberOfPods)
	assert.Equal(t, 2, result)

	pds.Spec.Strategy.MinAvailable = 2
	result = getEffectiveDisposalConcurrency(pds, numberOfPods)
	assert.Equal(t, 1, result)

	pds.Spec.Strategy.MinAvailable = 3
	result = getEffectiveDisposalConcurrency(pds, numberOfPods)
	assert.Equal(t, 0, result)

	pds.Spec.Strategy.MinAvailable = 4
	result = getEffectiveDisposalConcurrency(pds, numberOfPods)
	assert.Equal(t, 0, result)
}
