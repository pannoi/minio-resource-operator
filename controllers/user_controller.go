package controllers

import (
	"context"
	"math/rand"
	"net/url"
	"os"
	"strings"

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
		conditions := metav1.Condition{
			Status: "Failed",
			Reason: "Failed to connect to minio",
		}
		user.Status.Conditions = append(user.Status.Conditions, conditions)
		err = r.Status().Update(ctx, user)
		if err != nil {
			log.Error(err, "Failed to update status conditions")
			return ctrl.Result{}, err
		}
		log.Error(err, "Failed to connect to minio: "+minioEndpoint)
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
		conditions := metav1.Condition{
			Status: "Failed",
			Reason: "Failed to create user in minio",
		}
		user.Status.Conditions = append(user.Status.Conditions, conditions)
		err = r.Status().Update(ctx, user)
		if err != nil {
			log.Error(err, "Failed to update status conditions")
			return ctrl.Result{}, err
		}
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

	// FIXME: Ignore if secret already exists

	err = r.Create(ctx, secret, &client.CreateOptions{})
	if err != nil {
		conditions := metav1.Condition{
			Status: "Failed",
			Reason: "Failed to create secret",
		}
		user.Status.Conditions = append(user.Status.Conditions, conditions)
		err = r.Status().Update(ctx, user)
		if err != nil {
			log.Error(err, "Failed to update status conditions")
			return ctrl.Result{}, err
		}
		log.Error(err, "Failed to create secret with credentials: "+username)
		return ctrl.Result{Requeue: true}, err
	}

	if len(user.Spec.Policies) > 0 {
		for _, el := range user.Spec.Policies {
			err = mc.SetPolicy(ctx, el, username, false)
			if err != nil {
				conditions := metav1.Condition{
					Status: "Failed",
					Reason: "Failed to attach policy",
				}
				user.Status.Conditions = append(user.Status.Conditions, conditions)
				err = r.Status().Update(ctx, user)
				if err != nil {
					log.Error(err, "Failed to update status conditions")
					return ctrl.Result{}, err
				}
				log.Error(err, "Failed to attach policy: "+el+" to user "+username)
			}
		}
	}

	conditions := metav1.Condition{
		Status: "Ready",
		Reason: "Ready",
	}
	user.Status.Conditions = append(user.Status.Conditions, conditions)
	err = r.Status().Update(ctx, user)
	if err != nil {
		log.Error(err, "Failed to update status conditions")
		return ctrl.Result{}, err
	}

	log.Info("User was created: " + username)
	return ctrl.Result{}, nil
}

func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pannoiv1beta1.User{}).
		Complete(r)
}
