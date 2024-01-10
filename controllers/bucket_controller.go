/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"os"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pannoiv1beta1 "minio-resource-operator/api/v1beta1"
)

// BucketReconciler reconciles a Bucket object
type BucketReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=pannoi,resources=buckets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pannoi,resources=buckets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pannoi,resources=buckets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Bucket object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *BucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	bucket := &pannoiv1beta1.Bucket{}
	err := r.Get(ctx, req.NamespacedName, bucket)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Bucket resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Bucket resource")
		return ctrl.Result{}, err
	}

	mc, err := minio.New(os.Getenv("MINIO_ENDPOINT"), &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"), ""),
		Secure: false,
	})
	if err != nil {
		log.Error(err, "Failed to connect to minio: "+os.Getenv("MINIO_ENDPOINT"))
		return ctrl.Result{}, err
	}

	found, err := mc.BucketExists(ctx, bucket.Spec.Name)
	if err != nil {
		log.Error(err, "Cannot check if bucket exists")
		return ctrl.Result{}, err
	}
	if found {
		log.Info("Bucket already exists")
		return ctrl.Result{Requeue: false}, err
	}

	err = mc.MakeBucket(context.Background(), bucket.Name, minio.MakeBucketOptions{ObjectLocking: bucket.Spec.ObjectLocking.Enabled})
	if err != nil {
		log.Error(err, "Failed to create bucket: "+bucket.Spec.Name)
		return ctrl.Result{Requeue: true}, err
	}

	if bucket.Spec.ObjectLocking.Enabled {
		var retentionMode minio.RetentionMode
		validityUnit := minio.Days
		retentionPeriod := uint(bucket.Spec.ObjectLocking.Retention)

		switch strings.ToLower(bucket.Spec.ObjectLocking.Mode) {
		case "governance":
			retentionMode = minio.Governance
		case "compliance":
			retentionMode = minio.Compliance
		default:
			retentionMode = minio.Governance
		}

		err = mc.SetObjectLockConfig(ctx, bucket.Spec.Name, &retentionMode, &retentionPeriod, &validityUnit)
		if err != nil {
			log.Error(err, "Failed to enable object locking for: "+bucket.Spec.Name)
			return ctrl.Result{}, nil
		}
	}

	if bucket.Spec.Versioning.Enabled {
		err = mc.EnableVersioning(ctx, bucket.Spec.Name)
		if err != nil {
			log.Error(err, "Failed to enable bucket versioning: "+bucket.Spec.Name)
			return ctrl.Result{}, err
		}
	}

	log.Info("Minio bucket was created: " + bucket.Spec.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pannoiv1beta1.Bucket{}).
		Complete(r)
}
