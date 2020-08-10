/*
Copyright 2020 Fairwinds

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License
*/

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kuiperv1alpha1 "github.com/fairwindsops/kuiper/api/v1alpha1"
)

// AlternateImageSourceReconciler reconciles a AlternateImageSource object
type AlternateImageSourceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kuiper.fairwinds.com,resources=alternateimagesources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kuiper.fairwinds.com,resources=alternateimagesources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;update;patch;watch;list
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;watch;list

// Reconcile loads and reconciles the AlternateImageSource
func (r *AlternateImageSourceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("alternateimagesource", req.NamespacedName)

	var alternateImageSource kuiperv1alpha1.AlternateImageSource
	if err := r.Get(ctx, req.NamespacedName, &alternateImageSource); err != nil {
		log.Error(err, "unable to fetch AlternateImageSource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	alternateImageSource.Status.ObservedGeneration = alternateImageSource.ObjectMeta.Generation

	var deploymentsInNamespace appsv1.DeploymentList
	if err := r.List(ctx, &deploymentsInNamespace, client.InNamespace(req.Namespace)); err != nil {
		log.Error(err, "unable to list target deployments")
	}

	// Remove any targets that don't have existing deployments
	alternateImageSource.Status.TargetsAvailable = pruneTargetDeployments(alternateImageSource.Status.TargetsAvailable, deploymentsInNamespace)

	// Update Targets
	for _, replacement := range alternateImageSource.Spec.ImageSourceReplacements {
		for _, target := range replacement.Targets {
			switch strings.ToLower(target.Type.Kind) {
			//TODO: more types such as statefulset, daemonset
			case "deployment":
				for _, deployment := range deploymentsInNamespace.Items {
					if deployment.ObjectMeta.Name == target.Name {
						log.Info(fmt.Sprintf("found possible targeted deployment %s", deployment.ObjectMeta.Name))
						for _, container := range deployment.Spec.Template.Spec.Containers {
							repo, _, err := parseImageString(container.Image)
							if err != nil {
								log.Error(err, fmt.Sprintf("error parsing image for container %s", container.Name))
								continue
							}
							if stringInSlice(repo, replacement.EquivalentRepositories) {
								log.Info(fmt.Sprintf("found targeted image %s", container.Image))

								// Look to see if we are already tracking this deployment
								// if we are, just use that, if not then append it and start
								// using it
								var realTarget *kuiperv1alpha1.Target
								statusExists := false
								for _, statusTarget := range alternateImageSource.Status.TargetsAvailable {
									if statusTarget.UID == deployment.UID && container.Name == statusTarget.Container {
										statusExists = true
										realTarget = statusTarget
									}
								}
								if !statusExists {
									realTarget = &kuiperv1alpha1.Target{
										Name:              deployment.Name,
										Type:              target.Type,
										UID:               deployment.UID,
										Container:         container.Name,
										CurrentRepository: repo,
									}
									alternateImageSource.Status.TargetsAvailable = append(alternateImageSource.Status.TargetsAvailable, realTarget)
								}

								if r.targetNeedsActivation(realTarget, req.Namespace) {
									err := r.switchTarget(realTarget, req.Namespace, replacement)
									if err != nil {
										log.Error(err, "unable to update target")
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if err := r.Status().Update(ctx, &alternateImageSource); err != nil {
		log.Error(err, "unable to update AlternateImageSource status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// DeploymentToAlternateImageSource is a handler.ToRequestsFunc to be used to enqueue requests to reconcile deployments
// When a deployment is updated, this will request a reconciliation all of the AlternateImageSources in the namespace
// of the deployment that was updated.
func (r *AlternateImageSourceReconciler) DeploymentToAlternateImageSource(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}
	d, ok := o.Object.(*appsv1.Deployment)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Deployment but got a %T", o.Object), "failed to get AlternateImageSource for Deployment")
	}
	log := r.Log.WithValues("Deployment", d.Name, "Namespace", d.Namespace)
	log.V(3).Info("reconciling", "deployment")
	result = r.requestAISInNamespace(d.Namespace)

	return result
}

// PodToAlternateImageSource is a handler.ToRequestsFunc to be used to enqueue requests to reconcile Pods
// When a pod has an imagePullErr, this will request a reconciliation
func (r *AlternateImageSourceReconciler) PodToAlternateImageSource(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}
	ctx := context.Background()

	p, ok := o.Object.(*corev1.Pod)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Pod but got a %T", o.Object), "failed to get AlternateImageSource for Pod")
	}

	pod := &corev1.Pod{}
	err := r.Get(ctx, client.ObjectKey{Name: p.Name, Namespace: p.Namespace}, pod)
	if err != nil {
		return nil
	}

	if r.podHasImagePullErr(pod) {
		result = r.requestAISInNamespace(p.Namespace)
	}

	return result
}

// SetupWithManager sets up the reconciler
func (r *AlternateImageSourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kuiperv1alpha1.AlternateImageSource{}).
		Watches(
			&source.Kind{Type: &appsv1.Deployment{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.DeploymentToAlternateImageSource)},
		).Watches(
		&source.Kind{Type: &corev1.Pod{}},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.PodToAlternateImageSource)},
	).
		Complete(r)
}

func (r *AlternateImageSourceReconciler) podHasImagePullErr(pod *corev1.Pod) bool {
	log := r.Log.WithValues("Pod", pod.ObjectMeta.Name, "Namespace", pod.ObjectMeta.Namespace)
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting == nil {
			continue
		}
		if status.State.Waiting.Reason == "ErrImagePull" || status.State.Waiting.Reason == "ImagePullBackOff" {
			log.Info("pod has image pull issues")
			return true
		}
	}
	return false
}

func (r *AlternateImageSourceReconciler) requestAISInNamespace(namespace string) []ctrl.Request {
	log := r.Log.WithValues("requestAISInNamespace", namespace)
	alternateImageSourcesInNamespace := kuiperv1alpha1.AlternateImageSourceList{}
	result := []ctrl.Request{}
	err := r.List(context.Background(), &alternateImageSourcesInNamespace, client.InNamespace(namespace))
	if err != nil {
		log.Error(err, "error getting AlternateImageSources in namespace")
		return nil
	}
	for _, ais := range alternateImageSourcesInNamespace.Items {
		name := client.ObjectKey{Namespace: ais.Namespace, Name: ais.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}
	return result
}

// targetNeedsActivation detects if we need to activate a switch on that target
func (r *AlternateImageSourceReconciler) targetNeedsActivation(target *kuiperv1alpha1.Target, namespace string) bool {
	log := r.Log.WithValues("needsActivation", namespace)
	var podsInNamespace corev1.PodList
	if err := r.List(context.Background(), &podsInNamespace, client.InNamespace(namespace)); err != nil {
		log.Error(err, "unable to list pods in namespace")
		return false
	}

	for _, pod := range podsInNamespace.Items {
		if r.podHasImagePullErr(&pod) {
			for _, owner := range pod.OwnerReferences {
				// TODO: more types
				switch owner.Kind {
				case "ReplicaSet":
					rsOwner := r.getReplicaSetOwner(owner, namespace)
					if rsOwner.Kind == "Deployment" {
						if rsOwner.Name == target.Name {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// getReplicaSetOwner finds the owner of a replicaset's owner. We hope this is a deployment
func (r *AlternateImageSourceReconciler) getReplicaSetOwner(originalOwner v1.OwnerReference, namespace string) v1.OwnerReference {
	ctx := context.Background()
	rs := &appsv1.ReplicaSet{}
	err := r.Get(ctx, client.ObjectKey{Name: originalOwner.Name, Namespace: namespace}, rs)
	if err != nil {
		r.Log.Error(err, "could not find replicaset from owner")
		return v1.OwnerReference{}
	}
	if len(rs.OwnerReferences) == 1 {
		return rs.OwnerReferences[0]
	}

	return v1.OwnerReference{}
}

func (r *AlternateImageSourceReconciler) switchTarget(target *kuiperv1alpha1.Target, namespace string, replacement kuiperv1alpha1.ImageSourceReplacement) error {
	ctx := context.Background()
	target.SwitchStatuses = pruneSwitchStatus(target.SwitchStatuses)
	var newStatus = kuiperv1alpha1.SwitchStatus{
		Time: v1.Now(),
	}
	switch strings.ToLower(target.Type.Kind) {
	case "deployment":
		deployment := &appsv1.Deployment{}
		err := r.Get(ctx, client.ObjectKey{Name: target.Name, Namespace: namespace}, deployment)
		if err != nil {
			return err
		}

		for _, container := range deployment.Spec.Template.Spec.Containers {
			newStatus.OldImage, _, err = parseImageString(container.Image)
			if err != nil {
				r.Log.Error(err, fmt.Sprintf("could not parse conatainer %s", container.Name))
				continue
			}
			if stringInSlice(newStatus.OldImage, replacement.EquivalentRepositories) {
				for _, repository := range replacement.EquivalentRepositories {
					if !strings.Contains(container.Image, repository) {
						newStatus.NewImage = repository

						// TODO: make this check a separate function
						shouldSwitch := true
						for _, status := range target.SwitchStatuses {
							timeSinceSwitch := newStatus.Time.Sub(status.Time.Time)
							if timeSinceSwitch < time.Second*5 {
								// we are trying to switch too fast
								// TODO: implement a backoff
								r.Log.Info("not switching images in less than 5 seconds")
								shouldSwitch = false
							}
							if newStatus.NewImage == status.OldImage {
								// we are trying to switch back, maybe there is something wrong?
								if timeSinceSwitch < time.Minute {
									// we are trying to switch back in < 1 minute. Definitely seems bad
									r.Log.Info("detected a switch-back in < 1 minute")
									shouldSwitch = false
								}
							}
						}
						if shouldSwitch {
							r.Log.Info(fmt.Sprintf("switching from %s to %s", newStatus.OldImage, newStatus.NewImage))
							err = r.updateDeployment(deployment, newStatus.OldImage, newStatus.NewImage)
							if err != nil {
								r.Log.Error(err, "")
								continue
							}
							target.SwitchStatuses = append(target.SwitchStatuses, newStatus)
						}
					}
				}
			}
		}
	}
	return nil
}

func pruneSwitchStatus(statuses []kuiperv1alpha1.SwitchStatus) []kuiperv1alpha1.SwitchStatus {
	now := v1.Now()
	var returnList []kuiperv1alpha1.SwitchStatus
	count := 0
	for _, status := range statuses {
		age := now.Sub(status.Time.Time)
		if age < (time.Minute * 30) {
			returnList = append(returnList, status)
			count++
			if count >= 100 {
				return returnList
			}
		}
	}
	return returnList
}

func pruneTargetDeployments(targets []*kuiperv1alpha1.Target, deployments appsv1.DeploymentList) []*kuiperv1alpha1.Target {
	prunedList := []*kuiperv1alpha1.Target{}
	for _, deployment := range deployments.Items {
		for _, target := range targets {
			if target.UID == deployment.UID {
				prunedList = append(prunedList, target)
			}
		}
	}
	return prunedList
}

func parseImageString(image string) (string, string, error) {
	parsed := strings.Split(image, ":")
	if len(parsed) != 2 {
		return "", "", fmt.Errorf("could not parse image string: %s", image)
	}
	return parsed[0], parsed[1], nil
}

func (r *AlternateImageSourceReconciler) updateDeployment(deployment *appsv1.Deployment, old string, new string) error {
	replacedAny := false
	for idx, container := range deployment.Spec.Template.Spec.Containers {
		existingImage := container.Image
		if strings.Contains(existingImage, old) {
			newImage := strings.Replace(existingImage, old, new, 1)
			deployment.Spec.Template.Spec.Containers[idx].Image = newImage
			replacedAny = true
		}
	}

	if replacedAny {
		ctx := context.Background()
		err := r.Update(ctx, deployment)
		if err != nil {
			return err
		}
	}
	return nil
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if a == b {
			return true
		}
	}
	return false
}
