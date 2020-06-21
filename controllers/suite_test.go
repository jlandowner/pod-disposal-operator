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
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	psov1 "pod-disposal-operator/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{envtest.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	fmt.Println("** BeforeSuite start")
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = psov1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	close(done)
	fmt.Println("** BeforeSuite end")
}, 60)

var _ = AfterSuite(func() {
	fmt.Println("** AfterSuite start")
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
	fmt.Println("** AfterSuite end")
})

// SetupTest will set up a testing environment.
// This includes:
// * creating a Namespace to be used during the test
// * creating a Deployment and Pods to be used during the test
//   (create Pods manually because Deployment & Replicaset controller is not implemented in envtest.)
// * starting the 'PodDisposalScheduleReconciler'
// * stopping the 'PodDisposalScheduleReconciler" after the test ends
// Call this function at the start of each of your tests.
func SetupTest(ctx context.Context) *corev1.Namespace {
	fmt.Println("SetupTest start")
	var stopCh chan struct{}
	ns := &corev1.Namespace{}
	deploy := &appsv1.Deployment{}
	podlist := &corev1.PodList{}
	deployLabel := map[string]string{"deploy": "test-deploy"}
	pds := &psov1.PodDisposalSchedule{ObjectMeta: metav1.ObjectMeta{
		Name:      "test-pds",
		Namespace: ns.Name,
	}}

	BeforeEach(func() {
		fmt.Println("BeforeEach start")
		stopCh = make(chan struct{})

		// create test namespace
		*ns = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "testns-" + randStringRunes(5)},
		}
		err := k8sClient.Create(ctx, ns)
		Expect(err).NotTo(HaveOccurred(), "failed to create test namespace")

		podspec := corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx:alpine",
				},
			},
		}

		// create test deployment
		replica := int32(3)
		*deploy = appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-deploy", Namespace: ns.Name},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replica,
				Selector: &metav1.LabelSelector{MatchLabels: deployLabel},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: deployLabel},
					Spec:       podspec,
				},
			},
		}
		err = k8sClient.Create(ctx, deploy)
		Expect(err).NotTo(HaveOccurred(), "failed to create test deploy")

		// create test pods
		for i := 0; i < 3; i++ {
			pod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("test-deploy-pod-%d", i), Namespace: ns.Name, Labels: deployLabel},
				Spec:       podspec,
			}
			podlist.Items = append(podlist.Items, pod)
			time.Sleep(time.Second)
			err = k8sClient.Create(ctx, &pod)
			Expect(err).NotTo(HaveOccurred(), "failed to create test pod")
		}

		// PodDisposalScheduleReconciler manager start
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		err = (&PodDisposalScheduleReconciler{
			Client: mgr.GetClient(),
			Log:    ctrl.Log.WithName("controllers").WithName("PodDisposalSchedule"),
			Scheme: mgr.GetScheme(),
		}).SetupWithManager(mgr)

		Expect(err).NotTo(HaveOccurred(), "failed to setup controller")

		go func() {
			err := mgr.Start(stopCh)
			Expect(err).NotTo(HaveOccurred(), "failed to start manager")
		}()

		fmt.Println("BeforeEach end")
	})

	AfterEach(func() {
		fmt.Println("AfterEach start")
		close(stopCh)

		_ = k8sClient.Delete(ctx, pds)

		for _, pod := range podlist.Items {
			_ = k8sClient.Delete(ctx, &pod)
		}
		_ = k8sClient.Delete(ctx, deploy)

		err := k8sClient.Delete(ctx, ns)
		Expect(err).NotTo(HaveOccurred(), "failed to delete test namespace")

		fmt.Println("AfterEach end")
	})

	fmt.Println("SetupTest end")
	return ns
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func waitJustNextMinute() {
	now := time.Now()
	wait := now.Truncate(time.Minute).Add(time.Minute + time.Second)
	time.Sleep(wait.Sub(now))
}
