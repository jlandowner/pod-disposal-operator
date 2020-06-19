package controllers

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	psov1 "pod-disposal-operator/api/v1"

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

func TestFilterTargetPod(t *testing.T) {
	var pod1 corev1.Pod
	var pod2 corev1.Pod
	var pod3 corev1.Pod
	pod1.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC))
	pod2.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Date(2020, 1, 1, 2, 0, 0, 0, time.UTC))
	pod3.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Date(2020, 1, 1, 3, 0, 0, 0, time.UTC))

	var randamTimestampPodsSize3 corev1.PodList
	randamTimestampPodsSize3.Items = append(randamTimestampPodsSize3.Items, pod2)
	randamTimestampPodsSize3.Items = append(randamTimestampPodsSize3.Items, pod1)
	randamTimestampPodsSize3.Items = append(randamTimestampPodsSize3.Items, pod3)

	var timesortAscTimestampPodsSize3 corev1.PodList
	timesortAscTimestampPodsSize3.Items = append(timesortAscTimestampPodsSize3.Items, pod1)
	timesortAscTimestampPodsSize3.Items = append(timesortAscTimestampPodsSize3.Items, pod2)
	timesortAscTimestampPodsSize3.Items = append(timesortAscTimestampPodsSize3.Items, pod3)

	// randamTimestampPodsSize3, numberOfPods=1
	tgt := filterTargetPods(randamTimestampPodsSize3, 1)
	assert.Equal(t, 1, len(tgt.Items))
	assert.Equal(t, pod1.ObjectMeta.CreationTimestamp.Time, tgt.Items[0].CreationTimestamp.Time)

	// randamTimestampPodsSize3, numberOfPods=3
	tgt = filterTargetPods(randamTimestampPodsSize3, 3)
	assert.Equal(t, 3, len(tgt.Items))

	for i, pod := range tgt.Items {
		assert.Equal(t, timesortAscTimestampPodsSize3.Items[i].CreationTimestamp.Time, pod.GetCreationTimestamp().Time)
	}

	// randamTimestampPodsSize3, numberOfPods=4
	tgt = filterTargetPods(randamTimestampPodsSize3, 4)
	assert.Equal(t, len(randamTimestampPodsSize3.Items), len(tgt.Items))

	for i, pod := range tgt.Items {
		assert.Equal(t, timesortAscTimestampPodsSize3.Items[i].CreationTimestamp.Time, pod.GetCreationTimestamp().Time)
	}

}

func TestGetEffectiveDisposalConcurrency(t *testing.T) {
	var result int
	pds := psov1.PodDisposalSchedule{}
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
