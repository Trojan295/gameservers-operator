package teamspeak

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	chinchillav1 "gitlab.com/chinchilla-games/gameservers-operator/pkg/apis/chinchilla/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_teamspeak")

func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileTeamspeak{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New("teamspeak-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &chinchillav1.Teamspeak{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &chinchillav1.Teamspeak{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &chinchillav1.Teamspeak{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileTeamspeak{}

type ReconcileTeamspeak struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileTeamspeak) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Teamspeak")

	instance := &chinchillav1.Teamspeak{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if instance.Status.Address == "" {
		node, ipAddress := r.findFreeIpAddress(instance, reqLogger)
		if node == nil {
			return reconcile.Result{}, errors.NewTooManyRequestsError("cannot find free IP address")
		}

		instance.Status.Address = ipAddress
		if err := r.client.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, err
		}

	}

	if err := r.reconcilePod(instance, reqLogger); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileService(instance, reqLogger); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileTeamspeak) findFreeIpAddress(instance *chinchillav1.Teamspeak, reqLogger logr.Logger) (*corev1.Node, string) {
	ipAddresses := make(map[string]*corev1.Node)

	nodes := &corev1.NodeList{}
	r.client.List(context.TODO(), nodes)

	for _, node := range nodes.Items {
		addressesStr := node.Annotations["chinchilla.gameservers.addresses"]
		for _, address := range strings.Split(addressesStr, ",") {
			ipAddresses[address] = &node
		}
	}

	services := &corev1.ServiceList{}
	r.client.List(context.TODO(), services)

	for _, svc := range services.Items {
		if _, ok := svc.Spec.Selector["chinchilla.gameserver.type"]; ok {
			delete(ipAddresses, svc.Spec.ExternalIPs[0])
		}
	}

	for ip, node := range ipAddresses {
		return node, ip
	}

	return nil, ""
}

func (r *ReconcileTeamspeak) reconcilePod(instance *chinchillav1.Teamspeak, reqLogger logr.Logger) error {
	pod := newPod(instance)
	if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
		return err
	}

	found := &corev1.Pod{}

	err := r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		err = r.client.Create(context.TODO(), pod)

		if err != nil {
			return err
		}

		if err != nil {
			return err
		}

		// Pod created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	return nil
}

func newPod(cr *chinchillav1.Teamspeak) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    getPodLabels(cr),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "teamspeak",
					Image: fmt.Sprintf("teamspeak:%s", cr.Spec.Version),
					Env: []corev1.EnvVar{
						{Name: "TS3SERVER_LICENSE", Value: "accept"},
					},
					Ports: []corev1.ContainerPort{
						{ContainerPort: 30033, Protocol: corev1.ProtocolTCP},
						{ContainerPort: 9987, Protocol: corev1.ProtocolUDP},
					},
				},
			},
		},
	}
}

func (r *ReconcileTeamspeak) reconcileService(instance *chinchillav1.Teamspeak, reqLogger logr.Logger) error {
	service := newService(instance)
	if err := controllerutil.SetControllerReference(instance, service, r.scheme); err != nil {
		return err
	}

	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Service", "Service.Namespace", service.Namespace, "Service.Name", service.Name)

		err = r.client.Create(context.TODO(), service)

		if err != nil {
			return err
		}

		if err != nil {
			return err
		}

		return nil
	} else if err != nil {
		return err
	}

	reqLogger.Info("Skip reconcile: Service already exists", "Service.Namespace", found.Namespace, "Service.Name", found.Name)
	return nil
}

func newService(cr *chinchillav1.Teamspeak) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-svc",
			Namespace: cr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector:    getPodLabels(cr),
			ExternalIPs: []string{cr.Status.Address},
			Ports: []corev1.ServicePort{
				{
					Name:     "control",
					Protocol: corev1.ProtocolTCP,
					Port:     30033,
				},
				{
					Name:     "voice",
					Protocol: corev1.ProtocolUDP,
					Port:     9987,
				},
			},
		},
	}
}

func getPodLabels(cr *chinchillav1.Teamspeak) map[string]string {
	return map[string]string{
		"app":                        cr.Name,
		"chinchilla.gameserver.type": "teamspeak",
	}

}
