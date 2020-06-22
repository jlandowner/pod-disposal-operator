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
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	psov1 "pod-disposal-operator/api/v1"
)

var _ = Describe("PoddisposalscheduleController", func() {
	ctx := context.TODO()
	ns := SetupTest(ctx)

	Describe("Initialize", func() {
		It("should be placed default spec values, updated the status fields", func() {
			fmt.Println("Test start " + ns.Name)
			testpds := &psov1.PodDisposalSchedule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pds",
					Namespace: ns.Name,
				},
				Spec: psov1.PodDisposalScheduleSpec{
					Selector: psov1.Selector{
						Type: "Deployment",
						Name: "test-deploy",
					},
					Schedule: "*/2 * * * *",
					Strategy: psov1.Strategy{
						Order:    "Old",
						Lifespan: "3m",
					},
				},
			}

			err := k8sClient.Create(ctx, testpds)
			Expect(err).NotTo(HaveOccurred(), "failed to create test pds resource")

			time.Sleep(time.Second * 5)

			pds := &psov1.PodDisposalSchedule{}
			Eventually(
				getResourceFunc(ctx, client.ObjectKey{Name: "test-pds", Namespace: ns.Name}, pds),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			fmt.Println(pds)

			// Test default value is placed
			Expect(pds.Spec.Selector.Namespase).To(Equal(ns.Name))
			Expect(pds.Spec.Strategy.DisposalConcurrency).To(Equal(1))
			Expect(pds.Spec.Strategy.MinAvailable).To(Equal(1))
			Expect(pds.Status.LastDisposalCounts).To(Equal(0))
			Expect(pds.Status.LastDisposalTime.Time).To(Equal(getDefaultTime()))

			// Check pods is not deleted
			deploy := &appsv1.Deployment{}
			Eventually(
				getResourceFunc(ctx, client.ObjectKey{Name: "test-deploy", Namespace: ns.Name}, deploy),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			deploySelector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
			podlist := &corev1.PodList{}
			Eventually(
				listResourceFunc(ctx, podlist, client.MatchingLabelsSelector{Selector: deploySelector}),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			Expect(len(podlist.Items)).To(Equal(3))
		})
	})

	Describe("Normal case", func() {
		It("should behave that pod is deleted", func() {
			fmt.Println("Test start " + ns.Name)
			testpds := &psov1.PodDisposalSchedule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pds",
					Namespace: ns.Name,
				},
				Spec: psov1.PodDisposalScheduleSpec{
					Selector: psov1.Selector{
						Namespase: ns.Name,
						Type:      "Deployment",
						Name:      "test-deploy",
					},
					Schedule: "*/1 * * * *",
					Strategy: psov1.Strategy{
						Order:               "Old",
						Lifespan:            "1m",
						MinAvailable:        2,
						DisposalConcurrency: 2,
					},
				},
			}
			waitJustNextMinute()

			err := k8sClient.Create(ctx, testpds)
			Expect(err).NotTo(HaveOccurred(), "failed to create test pds resource")

			// Check Default
			time.Sleep(time.Second * 5)
			pds := &psov1.PodDisposalSchedule{}
			Eventually(
				getResourceFunc(ctx, client.ObjectKey{Name: "test-pds", Namespace: ns.Name}, pds),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			fmt.Println(pds)
			Expect(pds.Status.LastDisposalCounts).To(Equal(0))
			Expect(pds.Status.LastDisposalTime.Time).To(Equal(getDefaultTime()))

			// Test diposal should not start until 60s passed
			time.Sleep(time.Second * 30)
			Eventually(
				getResourceFunc(ctx, client.ObjectKey{Name: "test-pds", Namespace: ns.Name}, pds),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			fmt.Println(pds)
			Expect(pds.Status.LastDisposalCounts).To(Equal(0))
			Expect(pds.Status.LastDisposalTime.Time).To(Equal(getDefaultTime()))

			// Test LastDisposalCounts value should be modified after 60s passed
			time.Sleep(time.Second * 30)
			pds = &psov1.PodDisposalSchedule{}
			Eventually(
				getResourceFunc(ctx, client.ObjectKey{Name: "test-pds", Namespace: ns.Name}, pds),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			fmt.Println(pds)
			Expect(pds.Status.LastDisposalCounts).To(Equal(1))
			Expect(pds.Status.LastDisposalTime.Time).NotTo(Equal(getDefaultTime()))

			// Check a pod is deleted
			deploy := &appsv1.Deployment{}
			Eventually(
				getResourceFunc(ctx, client.ObjectKey{Name: "test-deploy", Namespace: ns.Name}, deploy),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			deploySelector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
			podlist := &corev1.PodList{}
			Eventually(
				listResourceFunc(ctx, podlist, client.MatchingLabelsSelector{Selector: deploySelector}),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			Expect(len(podlist.Items)).To(Equal(2))
			for _, pod := range podlist.Items {
				fmt.Println(pod.Name)
				Expect(pod.Name).NotTo(Equal("test-deploy-pod-0"))
			}
		})
	})

	Describe("when current number of pods is not satisfied with minAvailable", func() {
		It("should not dispose at all", func() {
			fmt.Println("Test start " + ns.Name)
			testpds := &psov1.PodDisposalSchedule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pds",
					Namespace: ns.Name,
				},
				Spec: psov1.PodDisposalScheduleSpec{
					Selector: psov1.Selector{
						Namespase: ns.Name,
						Type:      "Deployment",
						Name:      "test-deploy",
					},
					Schedule: "*/1 * * * *",
					Strategy: psov1.Strategy{
						Order:               "Old",
						Lifespan:            "1s",
						MinAvailable:        3,
						DisposalConcurrency: 2,
					},
				},
			}

			err := k8sClient.Create(ctx, testpds)
			Expect(err).NotTo(HaveOccurred(), "failed to create test pds resource")

			time.Sleep(time.Second * 65)
			// Test LastDisposalCounts is zero
			pds := &psov1.PodDisposalSchedule{}
			Eventually(
				getResourceFunc(ctx, client.ObjectKey{Name: "test-pds", Namespace: ns.Name}, pds),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			fmt.Println(pds)

			Expect(pds.Status.LastDisposalCounts).To(Equal(0))
			Expect(pds.Status.LastDisposalTime.Time).NotTo(Equal(getDefaultTime()))

			// Check pods are not deleted
			deploy := &appsv1.Deployment{}
			Eventually(
				getResourceFunc(ctx, client.ObjectKey{Name: "test-deploy", Namespace: ns.Name}, deploy),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			deploySelector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
			podlist := &corev1.PodList{}
			Eventually(
				listResourceFunc(ctx, podlist, client.MatchingLabelsSelector{Selector: deploySelector}),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			Expect(len(podlist.Items)).To(Equal(3))

		})
	})
})

func getResourceFunc(ctx context.Context, key client.ObjectKey, obj runtime.Object) func() error {
	return func() error {
		return k8sClient.Get(ctx, key, obj)
	}
}

func listResourceFunc(ctx context.Context, obj runtime.Object, opts client.ListOption) func() error {
	return func() error {
		return k8sClient.List(ctx, obj, opts)
	}
}
