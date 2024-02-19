package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/loft-sh/devpod-provider-kubernetes/pkg/throttledlogger"
	"github.com/loft-sh/devpod/pkg/command"
	perrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (k *KubernetesDriver) waitPodRunning(ctx context.Context, id string) (*corev1.Pod, error) {
	throttledLogger := throttledlogger.NewThrottledLogger(k.Log, time.Second*5)

	var pod *corev1.Pod
	err := wait.PollImmediate(time.Second, time.Minute*10, func() (bool, error) {
		var err error
		pod, err = k.getPod(ctx, id)
		if err != nil {
			return false, err
		} else if pod == nil {
			return true, nil
		}

		// check pod for problems
		if pod.DeletionTimestamp != nil {
			throttledLogger.Infof("Waiting, since pod '%s' is terminating", id)
			return false, nil
		}

		// check pod status
		if len(pod.Status.ContainerStatuses) < len(pod.Spec.Containers) {
			throttledLogger.Infof("Waiting, since pod '%s' is starting", id)
			return false, nil
		}

		// check container status
		for _, c := range pod.Status.InitContainerStatuses {
			containerStatus := &c
			if IsWaiting(containerStatus) {
				if IsCritical(containerStatus) {
					return false, fmt.Errorf("pod '%s' init container '%s' is waiting to start: %s (%s)", id, c.Name, c.State.Waiting.Message, c.State.Waiting.Reason)
				}

				throttledLogger.Infof("Waiting, since pod '%s' init container '%s' is waiting to start: %s (%s)", id, c.Name, c.State.Waiting.Message, c.State.Waiting.Reason)
				return false, nil
			}

			if IsTerminated(containerStatus) && !Succeeded(containerStatus) {
				return false, fmt.Errorf("pod '%s' init container '%s' is terminated: %s (%s)", id, c.Name, c.State.Terminated.Message, c.State.Terminated.Reason)
			}

			if IsRunning(containerStatus) {
				throttledLogger.Infof("Waiting, since pod '%s' init container '%s' is running", id, c.Name)
				return false, nil
			}
		}

		// check container status
		for _, c := range pod.Status.ContainerStatuses {
			containerStatus := &c
			// delete succeeded pods
			if IsTerminated(containerStatus) && Succeeded(containerStatus) {
				// delete pod that is succeeded
				k.Log.Debugf("Delete Pod '%s' because it is succeeded", id)
				err = k.deletePod(ctx, id)
				if err != nil {
					return false, err
				}

				return false, nil
			}

			if IsWaiting(containerStatus) {
				if IsCritical(containerStatus) {
					return false, fmt.Errorf("pod '%s' container '%s' is waiting to start: %s (%s)", id, c.Name, c.State.Waiting.Message, c.State.Waiting.Reason)
				}

				throttledLogger.Infof("Waiting, since pod '%s' container '%s' is waiting to start: %s (%s)", id, c.Name, c.State.Waiting.Message, c.State.Waiting.Reason)
				return false, nil
			}

			if IsTerminated(containerStatus) {
				return false, fmt.Errorf("pod '%s' container '%s' is terminated: %s (%s)", id, c.Name, c.State.Terminated.Message, c.State.Terminated.Reason)
			}

			if !IsReady(containerStatus) {
				throttledLogger.Infof("Waiting, since pod '%s' container '%s' is not ready yet", id, c.Name)
				return false, nil
			}
		}

		return true, nil
	})
	return pod, err
}

func (k *KubernetesDriver) getPod(ctx context.Context, id string) (*corev1.Pod, error) {
	// try to find pod
	out, err := k.buildCmd(ctx, []string{"get", "pod", id, "--ignore-not-found", "-o", "json"}).Output()
	if err != nil {
		return nil, fmt.Errorf("find container: %w", command.WrapCommandError(out, err))
	} else if len(out) == 0 {
		return nil, nil
	}

	// try to unmarshal pod
	pod := &corev1.Pod{}
	err = json.Unmarshal(out, pod)
	if err != nil {
		return nil, perrors.Wrap(err, "unmarshal pod")
	}

	return pod, nil
}

func (k *KubernetesDriver) waitPodDeleted(ctx context.Context, id string) error {
	out, err := k.buildCmd(ctx, []string{"delete", "pod", id, "--ignore-not-found", "--wait"}).Output()
	if err != nil {
		return fmt.Errorf("delete pod: %w", command.WrapCommandError(out, err))
	}

	return nil
}
