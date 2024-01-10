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
	"math/rand"
	"os"

	"github.com/minio/madmin-go"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pannoiv1beta1 "minio-resource-operator/api/v1beta1"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func generatePassword(l int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	s := make([]rune, l)
	for i := range s {
		s[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	return string(s)
}

//+kubebuilder:rbac:groups=pannoi,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pannoi,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pannoi,resources=users/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the User object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	user := &pannoiv1beta1.User{}
	err := r.Get(ctx, req.NamespacedName, user)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("User resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get User resource")
		return ctrl.Result{}, err
	}

	mc, err := madmin.New(
		os.Getenv("MINIO_ENDPOINT"),
		os.Getenv("MINIO_ACCESS_KEY"),
		os.Getenv("MINIO_SECRET_KEY"),
		false,
	)
	if err != nil {
		log.Error(err, "Failed to connect to minio: "+os.Getenv("MINIO_ENDPOINT"))
		return ctrl.Result{}, err
	}

	err = r.Get(ctx, req.NamespacedName, user)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("User not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get User resource")
		return ctrl.Result{}, err
	}

	username := user.Spec.Name
	password := generatePassword(20)

	err = mc.AddUser(ctx, username, password)
	if err != nil {
		log.Error(err, "Failed to create user: "+username)
		return ctrl.Result{Requeue: true}, nil
	}

	secretMap := make(map[string][]byte)
	secretMap["accessKey"] = []byte(username)
	secretMap["secretKey"] = []byte(password)

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      username + "-minio-credentials",
			Namespace: req.Namespace,
		},
		Type: corev1.SecretType("generic"),
		Data: secretMap,
	}

	err = r.Create(ctx, secret, &client.CreateOptions{})
	if err != nil {
		log.Error(err, "Failed to create secret with credentials: "+username)
		return ctrl.Result{Requeue: true}, err
	}

	if len(user.Spec.Policies) > 0 {
		for _, el := range user.Spec.Policies {
			err = mc.SetPolicy(ctx, el, username, false)
			if err != nil {
				log.Error(err, "Failed to attach policy: "+el+" to user "+username)
			}
		}
	}

	log.Info("User was created: " + username)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pannoiv1beta1.User{}).
		Complete(r)
}
