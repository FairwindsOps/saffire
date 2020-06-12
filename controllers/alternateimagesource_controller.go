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
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

// Reconcile loads and reconciles the AlternateImageSource
func (r *AlternateImageSourceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("alternateimagesource", req.NamespacedName)

	var alternateImageSource kuiperv1alpha1.AlternateImageSource

	if err := r.Get(ctx, req.NamespacedName, &alternateImageSource); err != nil {

		log.Error(err, "unable to fetch AlternateImageSource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if alternateImageSource.Status.ObservedGeneration == alternateImageSource.ObjectMeta.Generation {
		// Status is updated, metadata.generation of
		// CR is not incremented, no need to reconcile
		return ctrl.Result{}, nil
	}

	alternateImageSource.Status.Activated = false
	alternateImageSource.Status.ObservedGeneration = alternateImageSource.ObjectMeta.Generation

	for _, replacement := range alternateImageSource.Spec.ImageSourceReplacements {
		for _, target := range replacement.Targets {
			switch strings.ToLower(target.Type.Kind) {
			case "deployment":
				var targetDeployments appsv1.DeploymentList

				// TODO: apply label selector
				if err := r.List(ctx, &targetDeployments, client.InNamespace("")); err != nil {
					log.Error(err, "unable to list target deployments")
				}
				if len(targetDeployments.Items) < 1 {
					log.Info("no deployments found matching label selector on ", req.Name)
				}
				for _, deployment := range targetDeployments.Items {
					log.Info(fmt.Sprintf("found deployment %s", deployment.ObjectMeta.Name))
					alternateImageSource.Status.Targets = append(alternateImageSource.Status.Targets, deployment.ObjectMeta.Name)
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

func (r *AlternateImageSourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kuiperv1alpha1.AlternateImageSource{}).
		Complete(r)
}
