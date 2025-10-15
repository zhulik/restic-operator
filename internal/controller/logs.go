package controller

import (
	"context"
	"fmt"
	"io"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getJobPodLogs(ctx context.Context, kubeclient client.Client, config *rest.Config, _ logr.Logger, job *batchv1.Job) (string, error) {
	var podList v1.PodList
	err := kubeclient.List(ctx, &podList,
		client.InNamespace(job.Namespace),
		client.MatchingLabels{"batch.kubernetes.io/job-name": job.Name})
	if err != nil {
		return "", fmt.Errorf("failed to list pods for job %s: %w", job.Name, err)
	}

	return getPodContainrLogs(ctx, config, &podList.Items[0])
}

func getPodContainrLogs(ctx context.Context, config *rest.Config, pod *v1.Pod) (string, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}

	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{})

	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to stream logs from pod %s: %w", pod.Name, err)
	}
	defer stream.Close() // nolint:errcheck

	logs, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("failed to read logs from pod %s: %w", pod.Name, err)
	}

	return string(logs), nil
}
