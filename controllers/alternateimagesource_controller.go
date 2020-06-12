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
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;update;patch;watch

// Reconcile loads and reconciles the AlternateImageSource
func (r *AlternateImageSourceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("alternateimagesource", req.NamespacedName)

	var alternateImageSource kuiperv1alpha1.AlternateImageSource

	if err := r.Get(ctx, req.NamespacedName, &alternateImageSource); err != nil {

		log.Error(err, "unable to fetch AlternateImageSource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	alternateImageSource.Status.Activated = false
	alternateImageSource.Status.ObservedGeneration = alternateImageSource.ObjectMeta.Generation
	alternateImageSource.Status.Targets = []kuiperv1alpha1.Target{}

	for _, replacement := range alternateImageSource.Spec.ImageSourceReplacements {
		for _, target := range replacement.Targets {
			switch strings.ToLower(target.Type.Kind) {
			case "deployment":
				var targetDeployments appsv1.DeploymentList

				if err := r.List(ctx, &targetDeployments, client.InNamespace(req.Namespace)); err != nil {
					log.Error(err, "unable to list target deployments")
				}
				if len(targetDeployments.Items) < 1 {
					continue
				}
				for _, deployment := range targetDeployments.Items {
					log.Info(fmt.Sprintf("found deployment %s", deployment.ObjectMeta.Name))
					if deployment.ObjectMeta.Name == target.Name {
						if !targetInList(target, alternateImageSource.Status.Targets) {
							alternateImageSource.Status.Targets = append(alternateImageSource.Status.Targets, target)
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
	ctx := context.Background()

	d, ok := o.Object.(*appsv1.Deployment)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Deployment but got a %T", o.Object), "failed to get AlternateImageSource for Deployment")
	}
	log := r.Log.WithValues("Deployment", d.Name, "Namespace", d.Namespace)

	log.V(3).Info("reconciling", "deployment")

	alternateImageSourcesInNamespace := kuiperv1alpha1.AlternateImageSourceList{}
	err := r.List(ctx, &alternateImageSourcesInNamespace, client.InNamespace(d.Namespace))
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

// SetupWithManager sets up the reconciler
func (r *AlternateImageSourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kuiperv1alpha1.AlternateImageSource{}).
		Watches(
			&source.Kind{Type: &appsv1.Deployment{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.DeploymentToAlternateImageSource)},
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
