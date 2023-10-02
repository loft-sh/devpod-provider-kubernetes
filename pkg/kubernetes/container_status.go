package kubernetes

import corev1 "k8s.io/api/core/v1"

type ContainerStatus struct {
	status *corev1.ContainerStatus
}

func (cs *ContainerStatus) IsReady() bool {
	return cs.status.Ready
}

func (cs *ContainerStatus) IsWaiting() bool {
	return cs.status.State.Waiting != nil
}

func (cs *ContainerStatus) IsTerminated() bool {
	return cs.status.State.Terminated != nil
}

func (cs *ContainerStatus) Succeeded() bool {
	return cs.status.State.Terminated != nil && cs.status.State.Terminated.ExitCode == 0
}

func (cs *ContainerStatus) IsRunning() bool {
	return cs.status.State.Running != nil
}

func (cs *ContainerStatus) IsCriticalStatus() bool {
	return criticalStatus[cs.status.State.Waiting.Reason]
}

// criticalStatus container status
var criticalStatus = map[string]bool{
	"Error":                      true,
	"Unknown":                    true,
	"ImagePullBackOff":           true,
	"CrashLoopBackOff":           true,
	"RunContainerError":          true,
	"ErrImagePull":               true,
	"CreateContainerConfigError": true,
	"InvalidImageName":           true,
}
