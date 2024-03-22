package controllers

import (
	"context"
	"net/url"
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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BucketReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

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

	var minioEndpoint string
	if strings.Contains(os.Getenv("MINIO_ENDPOINT"), "http") {
		minioHost, _ := url.Parse(os.Getenv("MINIO_ENDPOINT"))
		minioEndpoint = minioHost.Host
	} else {
		minioEndpoint = os.Getenv("MINIO_ENDPOINT")
	}

	mc, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"), ""),
		Secure: false,
	})
	if err != nil {
		conditions := metav1.Condition{
			Status: "Failed",
			Reason: "Failed to connect to Minio",
		}
		bucket.Status.Conditions = append(bucket.Status.Conditions, conditions)
		err = r.Status().Update(ctx, bucket)
		if err != nil {
			log.Error(err, "Failed to update Bucket status")
			return ctrl.Result{}, err
		}
		log.Error(err, "Failed to connect to minio: "+minioEndpoint)
		return ctrl.Result{}, err
	}

	found, err := mc.BucketExists(ctx, bucket.Spec.Name)
	if err != nil {
		conditions := metav1.Condition{
			Status: "Failed",
			Reason: "Failed to connect to Minio",
		}
		bucket.Status.Conditions = append(bucket.Status.Conditions, conditions)
		err = r.Status().Update(ctx, bucket)
		if err != nil {
			log.Error(err, "Failed to update Bucket status")
			return ctrl.Result{}, err
		}
		log.Error(err, "Cannot check if bucket exists")
		return ctrl.Result{}, err
	}
	if found {
		log.Info("Bucket already exists")
		return ctrl.Result{Requeue: false}, err
	}

	err = mc.MakeBucket(context.Background(), bucket.Name, minio.MakeBucketOptions{ObjectLocking: bucket.Spec.ObjectLocking.Enabled})
	if err != nil {
		conditions := metav1.Condition{
			Status: "Failed",
			Reason: "Failed to create bucket",
		}
		bucket.Status.Conditions = append(bucket.Status.Conditions, conditions)
		err = r.Status().Update(ctx, bucket)
		if err != nil {
			log.Error(err, "Failed to update Bucket status")
			return ctrl.Result{}, err
		}
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
			conditions := metav1.Condition{
				Status: "Failed",
				Reason: "Failed to enable object locking",
			}
			bucket.Status.Conditions = append(bucket.Status.Conditions, conditions)
			err = r.Status().Update(ctx, bucket)
			if err != nil {
				log.Error(err, "Failed to update Bucket status")
				return ctrl.Result{}, err
			}
			log.Error(err, "Failed to enable object locking for: "+bucket.Spec.Name)
			return ctrl.Result{}, nil
		}
	}

	if bucket.Spec.Versioning.Enabled {
		err = mc.EnableVersioning(ctx, bucket.Spec.Name)
		if err != nil {
			conditions := metav1.Condition{
				Status: "Failed",
				Reason: "Failed to enable bucket versioning",
			}
			bucket.Status.Conditions = append(bucket.Status.Conditions, conditions)
			err = r.Status().Update(ctx, bucket)
			if err != nil {
				log.Error(err, "Failed to update Bucket status")
				return ctrl.Result{}, err
			}
			log.Error(err, "Failed to enable bucket versioning: "+bucket.Spec.Name)
			return ctrl.Result{}, err
		}
	}

	conditions := metav1.Condition{
		Status: "Ready",
		Reason: "Ready",
	}
	bucket.Status.Conditions = append(bucket.Status.Conditions, conditions)
	err = r.Status().Update(ctx, bucket)
	if err != nil {
		log.Error(err, "Failed to update Bucket status")
		return ctrl.Result{}, err
	}

	log.Info("Minio bucket was created: " + bucket.Spec.Name)
	return ctrl.Result{}, nil
}

func (r *BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pannoiv1beta1.Bucket{}).
		Complete(r)
}
