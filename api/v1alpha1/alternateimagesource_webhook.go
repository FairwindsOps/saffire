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

package v1alpha1

import (
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var alternateimagesourcelog = logf.Log.WithName("alternateimagesource-resource")

func (r *AlternateImageSource) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-kuiper-fairwinds-com-v1alpha1-alternateimagesource,mutating=true,failurePolicy=fail,groups=kuiper.fairwinds.com,resources=alternateimagesources,verbs=create;update,versions=v1alpha1,name=malternateimagesource.kb.io

var _ webhook.Defaulter = &AlternateImageSource{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *AlternateImageSource) Default() {
	alternateimagesourcelog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
	// Currently there are no defaults that I can think of.
}

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-kuiper-fairwinds-com-v1alpha1-alternateimagesource,mutating=false,failurePolicy=fail,groups=kuiper.fairwinds.com,resources=alternateimagesources,versions=v1alpha1,name=valternateimagesource.kb.io

var _ webhook.Validator = &AlternateImageSource{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *AlternateImageSource) ValidateCreate() error {
	alternateimagesourcelog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *AlternateImageSource) ValidateUpdate(old runtime.Object) error {
	alternateimagesourcelog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *AlternateImageSource) ValidateDelete() error {
	alternateimagesourcelog.Info("validate delete", "name", r.Name)

	// If it has been activated, we should not delete this object until it gets deactivated
	if r.Status.Activated {
		return fmt.Errorf("Cannot delete an activated AlternateImageSource")
	}
	return nil
}

func (r *AlternateImageSource) validateAlternateImageSource() error {
	var allErrs field.ErrorList

	for ri, source := range r.Spec.ImageSourceReplacements {
		for ti, target := range source.Targets {
			validateTarget(&target, field.NewPath("spec").Child("imageSourceReplacements").Index(ri).Child("targets").Index(ti))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "kuiper.fairwinds.com", Kind: "alternateimagesources"},
		r.Name, allErrs)
}

func validateTarget(t *Target, fldPath *field.Path) *field.Error {
	if strings.ToLower(t.Type.Kind) != "deployment" {
		return field.Invalid(fldPath, t, "only deployments are supported as targets")
	}
	return nil
}
