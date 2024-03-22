package controllers

import (
	"context"
	"net/url"
	"os"
	"strings"

	"github.com/minio/madmin-go"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pannoiv1beta1 "minio-resource-operator/api/v1beta1"
)

type PolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	policy := &pannoiv1beta1.Policy{}
	err := r.Get(ctx, req.NamespacedName, policy)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Policy resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Policy resource")
		return ctrl.Result{}, err
	}

	var minioEndpoint string
	if strings.Contains(os.Getenv("MINIO_ENDPOINT"), "http") {
		minioHost, _ := url.Parse(os.Getenv("MINIO_ENDPOINT"))
		minioEndpoint = minioHost.Host
	} else {
		minioEndpoint = os.Getenv("MINIO_ENDPOINT")
	}

	mc, err := madmin.New(
		minioEndpoint,
		os.Getenv("MINIO_ACCESS_KEY"),
		os.Getenv("MINIO_SECRET_KEY"),
		false,
	)
	if err != nil {
		log.Error(err, "Failed to connect to minio: "+minioEndpoint)
		return ctrl.Result{}, err
	}

	err = mc.AddCannedPolicy(ctx, policy.Spec.Name, []byte(policy.Spec.Statement))
	if err != nil {
		log.Error(err, "Failed to create policy: "+policy.Spec.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	log.Info("Policy was created: " + policy.Spec.Name)
	return ctrl.Result{}, nil
}

func (r *PolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pannoiv1beta1.Policy{}).
		Complete(r)
}
