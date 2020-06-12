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

	// TODO: We set the lists to empty and rebuild each reconciliation. There's probably more efficient ways to do this.
	alternateImageSource.Status.TargetsAvailable = []kuiperv1alpha1.Target{}
	alternateImageSource.Status.TargetsActivated = []kuiperv1alpha1.Target{}

	var deploymentsInNamespace appsv1.DeploymentList
	if err := r.List(ctx, &deploymentsInNamespace, client.InNamespace(req.Namespace)); err != nil {
		log.Error(err, "unable to list target deployments")
	}

	// Update Targets
	for _, replacement := range alternateImageSource.Spec.ImageSourceReplacements {
		for _, target := range replacement.Targets {
			switch strings.ToLower(target.Type.Kind) {
			//TODO: more types such as statefulset, daemonset
			case "deployment":
				for _, deployment := range deploymentsInNamespace.Items {
					if deployment.ObjectMeta.Name == target.Name {
						log.Info(fmt.Sprintf("found targeted deployment %s", deployment.ObjectMeta.Name))
						alternateImageSource.Status.TargetsAvailable = append(alternateImageSource.Status.TargetsAvailable, target)
						if r.targetNeedsActivation(target, req.Namespace) {
							alternateImageSource.Status.TargetsActivated = append(alternateImageSource.Status.TargetsActivated, target)
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
	log := r.Log.WithValues("Pod", p.Name, "Namespace", p.Namespace)

	pod := &corev1.Pod{}
	err := r.Get(ctx, client.ObjectKey{Name: p.Name, Namespace: p.Namespace}, pod)
	if err != nil {
		log.Error(err, "could not get pod")
		return result
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

func targetInList(target kuiperv1alpha1.Target, list []kuiperv1alpha1.Target) bool {
	for _, b := range list {
		if b == target {
			return true
		}
	}
	return false
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

// targetNeedsActivation detects if we need to activate the switchover on that target
func (r *AlternateImageSourceReconciler) targetNeedsActivation(target kuiperv1alpha1.Target, namespace string) bool {
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
