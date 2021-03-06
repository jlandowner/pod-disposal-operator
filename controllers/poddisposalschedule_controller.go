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
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pdov1 "pod-disposal-operator/api/v1"
)

// PodDisposalScheduleReconciler reconciles a PodDisposalSchedule object
type PodDisposalScheduleReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	realClock
}

// SetupWithManager returns Controller to run
func (r *PodDisposalScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pdov1.PodDisposalSchedule{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=operator.k8s.jlandowner.com,resources=poddisposalschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.k8s.jlandowner.com,resources=poddisposalschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods/status,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile is main reconciliation logic.
func (r *PodDisposalScheduleReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("pod-disposal-schedule", req.NamespacedName)
	log.V(1).Info("Start reconcile")

	var pds pdov1.PodDisposalSchedule
	if err := r.Get(ctx, req.NamespacedName, &pds); err != nil {
		log.Error(err, "failed to fetch PodDisposalSchedule")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, ignoreNotFound(err)
	}

	nextDisposalTime := pds.Status.NextDisposalTime.Time

	// initialize PodDisposalSchedule status when first reconcile.
	if isDefaultTime(pds.Status.NextDisposalTime.Time) {
		next, err := r.initPdsStatus(ctx, &pds)
		if err != nil {
			r.errorEvent(ctx, &pds, err, "Init", "Failed to initialize")
			return ctrl.Result{}, err
		}
		r.logEvent(ctx, &pds, "Init", "Successfully initalized status")
		return ctrl.Result{RequeueAfter: next.Sub(r.Now())}, nil
	}

	// start disposal when it has reached already to NextDisposlTime.
	if !isTimingToDisposal(pds.Status.NextDisposalTime.Time, r.Now()) {
		log.V(1).Info("Skip disposal", "next", nextDisposalTime.String())
		return ctrl.Result{RequeueAfter: nextDisposalTime.Sub(r.Now())}, nil
	}

	log.V(0).Info("Starting", "next", nextDisposalTime.String(),
		"order", pds.Spec.Strategy.Order,
		"diposal-concurrency", pds.Spec.Strategy.DisposalConcurrency,
		"lifespan", pds.Spec.Strategy.Lifespan,
		"min-available", pds.Spec.Strategy.MinAvailable)

	// get target pods by selector.
	var pods corev1.PodList
	pods, err := r.getTargetPods(ctx, &pds)
	if err != nil {
		r.errorEvent(ctx, &pds, err, "Fetching", "Failed to fetch target pods")
		return ctrl.Result{}, err
	}

	// calculate effective disposalConcurrency
	disposalConcurrency := getEffectiveDisposalConcurrency(pds, len(pods.Items))
	if disposalConcurrency == 0 {
		r.logEvent(ctx, &pds, "Skip", "MinAvailable is not satisfied",
			"effective-disposal-concurrency", disposalConcurrency)

		// update the pds status field.
		next, err := r.updateStatus(ctx, &pds, 0)
		if err != nil {
			r.errorEvent(ctx, &pds, err, "Update", "Failed to update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: next.Sub(r.Now())}, nil
	}

	lifespan, err := time.ParseDuration(pds.Spec.Strategy.Lifespan)
	if err != nil {
		r.errorEvent(ctx, &pds, err, "Parsing", "Unable to parse lifespan to duration "+pds.Spec.Strategy.Lifespan)
		return ctrl.Result{}, nil
	}

	// livingEnoughPods is a list of pods that are living over their lifespan.
	livingEnoughPods := pods.DeepCopy()
	livingEnoughPods.Items = make([]corev1.Pod, 0)
	for _, pod := range pods.Items {
		isRunning := isRunning(&pod)
		isLivingEnough := isLivingEnough(lifespan, pod.CreationTimestamp.Time, r.Now())
		log.V(1).Info("- Target pod", "pod", pod.GetName(),
			"creation-timestamp", pod.ObjectMeta.GetCreationTimestamp().String(),
			"is-running", isRunning, "is-living-enough", isLivingEnough)

		if isRunning && isLivingEnough {
			livingEnoughPods.Items = append(livingEnoughPods.Items, pod)
		}
	}

	// sort & filter the target pod.
	sortedLivingEnoughPods := sortPodsByOrder(livingEnoughPods, pds.Spec.Strategy.Order)
	targetPods := slicePodsByNumber(sortedLivingEnoughPods, disposalConcurrency)

	log.V(1).Info("Disposal Details",
		"effective-disposal-concurrency", disposalConcurrency,
		"living-enough-pods-count", len(targetPods.Items))

	// execute disposal.
	var disposalCounts int
	for _, pod := range targetPods.Items {
		err := r.Delete(ctx, &pod)
		if err != nil {
			log.V(1).Info("Failed to delete pod", "pod", pod.GetName())
			if ignoreNotFound(err) != nil {
				r.errorEvent(ctx, &pds, err, "Disposing", fmt.Sprintf("Failed to delete pod %s created at %s", pod.GetName(), pod.GetCreationTimestamp()))
			}
		} else {
			r.logEvent(ctx, &pds, "Disposed", fmt.Sprintf("Successfully delete pod %s created at %s", pod.GetName(), pod.GetCreationTimestamp()))
			disposalCounts++
		}
	}

	if disposalCounts == 0 {
		r.logEvent(ctx, &pds, "NoDisposed", "No Pods are disposed")
	}

	// check if the number of deleted Pods is as expected.
	if len(targetPods.Items) != disposalCounts {
		r.errorEvent(ctx, &pds, fmt.Errorf("Pod disposal count is expected %d but actual %d", len(targetPods.Items), disposalCounts), "Disposed", "Failed to dispose")
	}

	// update the pds status field.
	next, err := r.updateStatus(ctx, &pds, disposalCounts)
	if err != nil {
		r.errorEvent(ctx, &pds, err, "Update", "Failed to update status")
		return ctrl.Result{}, err
	}

	log.V(0).Info("Finished", "disposal-counts", disposalCounts, "next", next.String())

	return ctrl.Result{RequeueAfter: next.Sub(r.Now())}, nil
}

// getTargetPods returns target pods by selector.
func (r *PodDisposalScheduleReconciler) getTargetPods(ctx context.Context, pds *pdov1.PodDisposalSchedule) (pods corev1.PodList, err error) {
	switch pds.Spec.Selector.Type {
	case "Deployment":
		deployName := pds.Spec.Selector.Name
		namespace := pds.Spec.Selector.Namespase

		var deploy appsv1.Deployment
		if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: deployName}, &deploy); err != nil {
			return pods, err
		}

		deploySelector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
		if err != nil {
			return pods, err
		}

		if err := r.List(ctx, &pods, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: deploySelector}); err != nil {
			return pods, err
		}

	default:
		return pods, fmt.Errorf("SelectorType Not Match %s", pds.Spec.Selector.Type)
	}

	return pods, nil
}

// initPdsStatus updates the default value and status in creation.
func (r *PodDisposalScheduleReconciler) initPdsStatus(ctx context.Context, pds *pdov1.PodDisposalSchedule) (nextDisposalTime time.Time, err error) {
	// set default value of DisposalConcurrency
	if pds.Spec.Strategy.DisposalConcurrency == 0 {
		pds.Spec.Strategy.DisposalConcurrency = 1
	}

	// set default value of MinAvailable
	if pds.Spec.Strategy.MinAvailable == 0 {
		pds.Spec.Strategy.MinAvailable = 1
	}

	// set default value of selector.Namespase
	if pds.Spec.Selector.Namespase == "" {
		pds.Spec.Selector.Namespase = pds.GetNamespace()
	}

	if err := r.Update(ctx, pds); err != nil {
		return getDefaultTime(), err
	}

	// set nextDisposalTime
	nextDisposalTime, err = getNextSchedule(pds.Spec.Schedule, r.Now())
	if err != nil {
		return getDefaultTime(), errors.New("unable to calculate next disposal time by cron schedule")
	}
	pds.Status.NextDisposalTime = metav1.NewTime(nextDisposalTime)

	// set default time into lastDisposalTime
	pds.Status.LastDisposalTime = metav1.NewTime(getDefaultTime())

	if err := r.Status().Update(ctx, pds); err != nil {
		return getDefaultTime(), err
	}

	return nextDisposalTime, nil
}

// updateStatus updates PodDisposalSchedule resource status with Next&Last Disposal Time and Last Disposal Counts
func (r *PodDisposalScheduleReconciler) updateStatus(ctx context.Context, pds *pdov1.PodDisposalSchedule, lastDisposalCounts int) (nextDisposalTime time.Time, err error) {
	// set nextDisposalTime
	nextDisposalTime, err = getNextSchedule(pds.Spec.Schedule, r.Now())
	if err != nil {
		return getDefaultTime(), errors.New("unable to calculate next disposal time by cron schedule")
	}
	pds.Status.NextDisposalTime = metav1.NewTime(nextDisposalTime)

	// set lastDisposalTime
	pds.Status.LastDisposalTime = metav1.NewTime(r.Now())

	// set lastDisposalCounts
	pds.Status.LastDisposalCounts = lastDisposalCounts

	if err := r.Status().Update(ctx, pds); err != nil {
		return getDefaultTime(), err
	}

	return nextDisposalTime, nil
}
