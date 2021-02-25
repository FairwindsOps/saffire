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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/fairwindsops/controller-utils/pkg/controller"
	saffirev1alpha1 "github.com/fairwindsops/saffire/api/v1alpha1"
)

// AlternateImageSourceReconciler reconciles a AlternateImageSource object
type AlternateImageSourceReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	RestMapper    meta.RESTMapper
	DynamicClient dynamic.Interface
}

// +kubebuilder:rbac:groups=saffire.fairwinds.com,resources=alternateimagesources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=saffire.fairwinds.com,resources=alternateimagesources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;watch;list
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;update

// Reconcile loads and reconciles the AlternateImageSource
func (r *AlternateImageSourceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("alternateimagesource", req.NamespacedName)

	var alternateImageSource saffirev1alpha1.AlternateImageSource
	if err := r.Get(ctx, req.NamespacedName, &alternateImageSource); err != nil {
		log.Error(err, "unable to fetch AlternateImageSource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	alternateImageSource.Status.ObservedGeneration = alternateImageSource.ObjectMeta.Generation
	alternateImageSource.Status.Switches = pruneSwitchStatus(alternateImageSource.Status.Switches)

	for _, replacement := range alternateImageSource.Spec.ImageSourceReplacements {
		newSwitchStatus, err := r.needsActivation(req.Namespace, replacement.EquivalentRepositories)
		if err != nil {
			return ctrl.Result{}, err
		}
		if newSwitchStatus == nil {
			continue
		}

		if alternateImageSource.Status.Switches != nil {
			if !r.shouldSwitch(alternateImageSource.Status.Switches, *newSwitchStatus) {
				return ctrl.Result{}, nil
			}
		}

		err = r.switchImage(newSwitchStatus, req.Namespace)
		if err != nil {
			log.Error(err, "unable to update target")
		}
		alternateImageSource.Status.Switches = append(alternateImageSource.Status.Switches, *newSwitchStatus)
	}

	if err := r.Status().Update(ctx, &alternateImageSource); err != nil {
		log.Error(err, "unable to update AlternateImageSource status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
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
		For(&saffirev1alpha1.AlternateImageSource{}).
		Watches(
			&source.Kind{Type: &corev1.Pod{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.PodToAlternateImageSource)},
		).
		Complete(r)
}

func (r *AlternateImageSourceReconciler) podHasImagePullErr(pod *corev1.Pod) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting == nil {
			continue
		}
		if status.State.Waiting.Reason == "ErrImagePull" || status.State.Waiting.Reason == "ImagePullBackOff" {
			return true
		}
	}
	return false
}

// requestAISInNamespace requests reconciliation of any AlternateImageSources in a namespace
func (r *AlternateImageSourceReconciler) requestAISInNamespace(namespace string) []ctrl.Request {
	log := r.Log.WithValues("requestAISInNamespace", namespace)
	alternateImageSourcesInNamespace := saffirev1alpha1.AlternateImageSourceList{}
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

// needsActivation finds the first pod and container in a namespace and returns them if they have an equivalent repository
// and if they have image pull issues. Also returns the new image string
func (r *AlternateImageSourceReconciler) needsActivation(namespace string, equivalentRepositories []string) (*saffirev1alpha1.SwitchStatus, error) {
	log := r.Log.WithValues("needsActivation", namespace)
	var podsInNamespace corev1.PodList
	if err := r.List(context.Background(), &podsInNamespace, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	for _, pod := range podsInNamespace.Items {
		if r.podHasImagePullErr(&pod) {
			log.Info("pod has image pull errors - checking for equivalentRepository")
			for _, container := range pod.Spec.Containers {
				repositoryName, _, err := parseImageString(container.Image)
				if err != nil {
					log.Error(err, "could not parse image string")
					continue
				}
				if funk.ContainsString(equivalentRepositories, repositoryName) {
					log.Info(fmt.Sprintf("container %s has equivalentRepositories for image %s", container.Name, container.Image))
					newImageString, err := getNewImage(container.Image, equivalentRepositories)
					if err != nil {
						return nil, err
					}

					controller := r.getPodController(&pod)

					switchStatus := &saffirev1alpha1.SwitchStatus{
						Time: v1.Now(),
						Target: saffirev1alpha1.Target{
							Name:      controller.GetName(),
							Container: container.Name,
							Type: v1.GroupKind{
								Kind:  controller.GetKind(),
								Group: controller.GetAPIVersion(),
							},
						},
						OldImage: container.Image,
						NewImage: newImageString,
					}

					return switchStatus, nil
				}
			}
		}
	}
	return nil, nil
}

// getPodController determines the top-level controller of a pod
func (r *AlternateImageSourceReconciler) getPodController(pod *corev1.Pod) *unstructured.Unstructured {
	log := r.Log.WithValues("getPodController", pod.Name)

	cache := make(map[string]unstructured.Unstructured)

	podData, err := json.Marshal(pod)
	if err != nil {
		log.Error(err, "unable to marshal pod")
		return nil
	}

	placeholder := make(map[string]interface{})
	err = json.Unmarshal(podData, &placeholder)
	if err != nil {
		log.Error(err, "could not unmarshal pod")
		return nil
	}

	unstructuredPod := unstructured.Unstructured{
		Object: placeholder,
	}

	controller, err := controller.GetTopController(context.TODO(), r.DynamicClient, r.RestMapper, unstructuredPod, cache)
	if err != nil {
		log.Error(err, "could not get top controller")
		return nil
	}
	log.Info("found controller", "kind", controller.GetKind(), "name", controller.GetName())
	return &controller
}

// switch image executes the switch for different controller types
func (r *AlternateImageSourceReconciler) switchImage(switchStatus *saffirev1alpha1.SwitchStatus, namespace string) error {
	ctx := context.Background()

	switch strings.ToLower(switchStatus.Target.Type.Kind) {
	case "deployment":
		deployment := &appsv1.Deployment{}
		err := r.Get(ctx, client.ObjectKey{Name: switchStatus.Target.Name, Namespace: namespace}, deployment)
		if err != nil {
			return err
		}

		for _, container := range deployment.Spec.Template.Spec.Containers {
			if container.Name == switchStatus.Target.Container {

				r.Log.Info("switching", "old", switchStatus.OldImage, "new", switchStatus.NewImage)
				err = r.updateDeployment(deployment, switchStatus.OldImage, switchStatus.NewImage)
				if err != nil {
					r.Log.Error(err, "")
					continue
				}
			}
		}
	default:
		r.Log.Info("controller type not implemented yet", "type", switchStatus.Target.Type.Kind)
	}

	return nil
}

// shouldSwitch determines if a switch is possible within time constraints
// This prevents switching back too fast, or switching too fast in general
func (r *AlternateImageSourceReconciler) shouldSwitch(oldSwitchStatuses []saffirev1alpha1.SwitchStatus, newSwitchStatus saffirev1alpha1.SwitchStatus) bool {
	//TODO: Implement a backoff instead of a strict time.
	delaySeconds := 60 * time.Second
	var latestSwitchStatus saffirev1alpha1.SwitchStatus
	now := v1.Now()
	for idx, switchStatus := range oldSwitchStatuses {
		if newSwitchStatus.Target.Name != switchStatus.Target.Name {
			if newSwitchStatus.Target.Type.Kind != switchStatus.Target.Type.Kind {
				// Not the same target, so we don't need to check it
				continue
			}
		}
		// Find the latest switch that happened so we can make sure we aren't going too fast
		if idx == 0 {
			latestSwitchStatus = switchStatus
		} else {
			if latestSwitchStatus.Time.Sub(switchStatus.Time.Time) < 0 {
				latestSwitchStatus = switchStatus
			}
		}

		// We should not switch images back within the delaySeconds time
		// This may not be necessary. We might just keep it to the delaySeconds
		// no matter which images we are switching to/from
		if switchStatus.NewImage == newSwitchStatus.OldImage {
			if switchStatus.OldImage == newSwitchStatus.NewImage {
				diff := now.Sub(switchStatus.Time.Time)
				if diff > delaySeconds {
					r.Log.Info("switch back detected in too short of a time")
					return false
				}
			}
		}
	}
	// Switch is happening too fast.
	if now.Sub(latestSwitchStatus.Time.Time) < delaySeconds {
		r.Log.Info("switch happening too quickly")
		return false
	}

	return true
}

func pruneSwitchStatus(statuses []saffirev1alpha1.SwitchStatus) []saffirev1alpha1.SwitchStatus {
	now := v1.Now()
	var returnList []saffirev1alpha1.SwitchStatus
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
