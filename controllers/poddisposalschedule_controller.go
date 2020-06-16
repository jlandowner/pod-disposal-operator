package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatork8sv1 "pod-disposal-operator/api/v1"
)

// PodDisposalScheduleReconciler reconciles a PodDisposalSchedule object
type PodDisposalScheduleReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=operator.k8s.jlandowner.com,resources=poddisposalschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.k8s.jlandowner.com,resources=poddisposalschedules/status,verbs=get;update;patch

func (r *PodDisposalScheduleReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("poddisposalschedule", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *PodDisposalScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatork8sv1.PodDisposalSchedule{}).
		Complete(r)
}
