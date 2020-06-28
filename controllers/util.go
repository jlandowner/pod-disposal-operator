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
	"time"

	"github.com/robfig/cron"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"

	pdov1 "pod-disposal-operator/api/v1"
)

var (
	initT    = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	defaultT = time.Date(2006, 1, 2, 0, 0, 0, 0, time.Local)
)

type realClock struct{}

// Now is the time for mock
func (realClock) Now() time.Time { return time.Now() }

// Clock knows how to get the current time.
// It can be used to fake out timing for testing.
type Clock interface {
	Now() time.Time
}

// ignoreNotFound return nil if the given err is NotFoundErr.
func ignoreNotFound(err error) error {
	if apierrs.IsNotFound(err) {
		return nil
	}
	return err
}

// getNextSchedule returns a cron format for next activation time against now.
func getNextSchedule(cronPattern string, now time.Time) (nextTime time.Time, err error) {
	sched, err := cron.ParseStandard(cronPattern)
	if err != nil {
		return time.Time{}, fmt.Errorf("Unparseable schedule %q: %v", cronPattern, err)
	}
	return sched.Next(now), nil
}

// getDefaultLocalTime returns default time.
func getDefaultTime() time.Time {
	return defaultT
}

// isDefaultTime returns true if time is the default time.
func isDefaultTime(t time.Time) bool {
	return t.Equal(defaultT) || t.Equal(initT)
}

// isTimingToDisposal returns true if archeving to NextDisposalTime.
func isTimingToDisposal(nextDisposalTime time.Time, now time.Time) bool {
	if isDefaultTime(nextDisposalTime) {
		return false
	}
	return nextDisposalTime.Before(now)
}

// isRunning returns true if pod status is running
// Based on https://github.com/kubernetes/kubernetes/blob/master/pkg/printers/internalversion/printers.go#L705
func isRunning(pod *corev1.Pod) bool {
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	// return false if all init containers have not finished.
	for i := range pod.Status.InitContainerStatuses {
		initContainer := pod.Status.InitContainerStatuses[i]
		if initContainer.State.Terminated == nil || initContainer.State.Terminated.ExitCode != 0 {
			return false
		}
	}

	hasRunning := false
	for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
		container := pod.Status.ContainerStatuses[i]
		if container.Ready && container.State.Running != nil {
			hasRunning = true
			break
		}
	}

	if reason == "Completed" && hasRunning {
		hasPodReadyCondition := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				hasPodReadyCondition = true
			}
		}
		// change pod status back to "Running" if there is at least one container still reporting as "Running" status
		if hasPodReadyCondition {
			reason = "Running"
		} else {
			reason = "NotReady"
		}
	}

	if pod.DeletionTimestamp != nil && pod.Status.Reason == "NodeLost" {
		reason = "Unknown"
	} else if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}

	return reason == "Running"
}

// isLivingEnough returns true if the time pod is alive (Now - pod.creationTimeStamp) is longer than lifespan.
func isLivingEnough(lifespan time.Duration, birth time.Time, now time.Time) bool {
	return birth.Add(lifespan).Before(now)
}

// slicePodsByNumber returns an slice of the number of pods specified by numberOfPods.
func slicePodsByNumber(pods *corev1.PodList, numberOfPods int) *corev1.PodList {
	newPods := pods.DeepCopy()
	if numberOfPods-len(pods.Items) > 0 {
		numberOfPods = len(pods.Items)
	}
	newPods.Items = pods.Items[0:numberOfPods]

	return newPods
}

// getEffectiveDisposalConcurrency returns a effective DisposalConcurrency
// effective DisposalConcurrency is smaller value of disposalConcurrency or maxNumberOfDisposal(pod count minus minAvailable pod count)
func getEffectiveDisposalConcurrency(pds pdov1.PodDisposalSchedule, numberOfTargetPods int) int {
	minAvailable := pds.Spec.Strategy.MinAvailable
	disposalConcurrency := pds.Spec.Strategy.DisposalConcurrency
	maxNumberOfDisposal := numberOfTargetPods - minAvailable

	if maxNumberOfDisposal <= 0 {
		return 0
	}

	if disposalConcurrency > maxNumberOfDisposal {
		return maxNumberOfDisposal
	}
	return disposalConcurrency
}
