package controllers

import (
	"context"
	"fmt"
	psov1 "pod-disposal-operator/api/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EventType is a type of Event object (events.k8s.io)
type EventType string

const (
	// Normal is a normal Event
	Normal EventType = "Normal"
	// Error is a error Event
	Error EventType = "Error"
)

// String retruns string format EventType
func (e EventType) String() string {
	return string(e)
}

// invokeEvent invoke event of the poddisposalschedule
func (r *PodDisposalScheduleReconciler) invokeEvent(ctx context.Context, pds *psov1.PodDisposalSchedule, eventType EventType, reason string, message string) error {
	now := r.realClock.Now()
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "pds-" + getUUID(), Namespace: pds.Namespace},
		Source: corev1.EventSource{
			Component: "pod-disposal-operator",
		},
		InvolvedObject: corev1.ObjectReference{
			APIVersion:      pds.APIVersion,
			Kind:            pds.Kind,
			Name:            pds.Name,
			Namespace:       pds.Namespace,
			ResourceVersion: pds.ResourceVersion,
			UID:             pds.UID,
		},
		Type:           eventType.String(),
		Reason:         reason,
		Message:        message,
		FirstTimestamp: metav1.NewTime(now),
		LastTimestamp:  metav1.NewTime(now),
	}
	return r.Create(ctx, event)
}

// logEvent logs messages and invoke event
func (r *PodDisposalScheduleReconciler) logEvent(ctx context.Context, pds *psov1.PodDisposalSchedule, reason string, message string, logKeysAndValues ...interface{}) {
	log := r.Log.WithValues("podDisposalSchedule", pds.GetName())
	logKeysAndValues = append(logKeysAndValues, "Reason", reason)
	log.V(0).Info(message, logKeysAndValues...)
	err := r.invokeEvent(ctx, pds, Normal, reason, message)
	if err != nil {
		log.Error(err, "failed to create event", pds.Name, message)
	}
}

// errorEvent logs error and invoke event
func (r *PodDisposalScheduleReconciler) errorEvent(ctx context.Context, pds *psov1.PodDisposalSchedule, err error, reason string, message string, logKeysAndValues ...interface{}) {
	log := r.Log.WithValues("podDisposalSchedule", pds.GetName())
	log.Error(err, message, logKeysAndValues...)
	ierr := r.invokeEvent(ctx, pds, Error, reason, fmt.Sprintf("%s: %s", message, err.Error()))
	if ierr != nil {
		log.Error(ierr, "failed to create event", pds.Name, message)
	}
}
