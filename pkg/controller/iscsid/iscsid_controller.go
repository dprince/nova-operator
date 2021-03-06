package iscsid

import (
        "context"

	novav1 "github.com/openstack-k8s-operators/nova-operator/pkg/apis/nova/v1"
        appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
        "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_iscsid")

// Add creates a new Iscsid Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileIscsid{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("iscsid-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Iscsid
	err = c.Watch(&source.Kind{Type: &novav1.Iscsid{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Iscsid
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &novav1.Iscsid{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileIscsid implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileIscsid{}

// ReconcileIscsid reconciles a Iscsid object
type ReconcileIscsid struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Iscsid object and makes changes based on the state read
// and what is in the Iscsid.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileIscsid) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Iscsid")

	// Fetch the Iscsid instance
	instance := &novav1.Iscsid{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

        // Define a new Daemonset object
        ds := newDaemonset(instance, instance.Name)

        // Set Iscsid instance as the owner and controller
        if err := controllerutil.SetControllerReference(instance, ds, r.scheme); err != nil {                                              
                return reconcile.Result{}, err
        }

        // Check if this Daemonset already exists
        found := &appsv1.DaemonSet{}
        err = r.client.Get(context.TODO(), types.NamespacedName{Name: ds.Name, Namespace: ds.Namespace}, found)
        if err != nil && errors.IsNotFound(err) {
                reqLogger.Info("Creating a new Daemonset", "Ds.Namespace", ds.Namespace, "Ds.Name", ds.Name)
                err = r.client.Create(context.TODO(), ds)
                if err != nil {
                        return reconcile.Result{}, err
                }

                // Daemonset created successfully - don't requeue
                return reconcile.Result{}, nil
        } else if err != nil {
                return reconcile.Result{}, err
        }

        // Daemonset already exists - don't requeue
        reqLogger.Info("Skip reconcile: Daemonset already exists", "Ds.Namespace", found.Namespace, "Ds.Name", found.Name)
        return reconcile.Result{}, nil
}

func newDaemonset(cr *novav1.Iscsid, cmName string) *appsv1.DaemonSet {
        var trueVar bool = true

        daemonSet := appsv1.DaemonSet{
                TypeMeta: metav1.TypeMeta{
                        Kind:       "DaemonSet",
                        APIVersion: "apps/v1",
                },
                ObjectMeta: metav1.ObjectMeta{
                        Name:      cmName,
                        Namespace: cr.Namespace,
                        //OwnerReferences: []metav1.OwnerReference{
                        //      *metav1.NewControllerRef(cr, schema.GroupVersionKind{
                        //              Group:   v1beta1.SchemeGroupVersion.Group,
                        //              Version: v1beta1.SchemeGroupVersion.Version,
                        //              Kind:    "GenericDaemon",
                        //      }),
                        //},
                },
                Spec: appsv1.DaemonSetSpec{
                        Selector: &metav1.LabelSelector{
                                MatchLabels: map[string]string{"daemonset": cr.Name + "-daemonset"},
                        },
                        Template: corev1.PodTemplateSpec{
                                ObjectMeta: metav1.ObjectMeta{
                                        Labels: map[string]string{"daemonset": cr.Name + "-daemonset"},
                                },
                                Spec: corev1.PodSpec{
                                        NodeSelector:   map[string]string{"daemon": cr.Spec.Label},
                                        HostNetwork:    true,
                                        HostPID:        true,
                                        DNSPolicy:      "ClusterFirstWithHostNet",
                                        Containers:     []corev1.Container{},
                                        Tolerations:    []corev1.Toleration{},
                                        ServiceAccountName: cr.Spec.ServiceAccount,
                                },
                        },
                },
        }

        tolerationSpec := corev1.Toleration{
                Operator: "Exists",
        }
        daemonSet.Spec.Template.Spec.Tolerations = append(daemonSet.Spec.Template.Spec.Tolerations, tolerationSpec)


        iscsiContainerSpec := corev1.Container{
                Name:  "iscsid",
                Image: cr.Spec.IscsidImage,
                //ReadinessProbe: &corev1.Probe{
                //        Handler: corev1.Handler{
                //                Exec: &corev1.ExecAction{
                //                        Command: []string{
                //                                "/openstack/healthcheck", "libvirtd",
                //                        },
                //                },
                //        },
                //        InitialDelaySeconds: 30,
                //        PeriodSeconds:       30,
                //        TimeoutSeconds:      1,
                //},
                Command: []string{
                        "/usr/sbin/iscsid", "-f",
                },
                SecurityContext: &corev1.SecurityContext{
                        Privileged:  &trueVar,
                },
                VolumeMounts: []corev1.VolumeMount{
                        {
                                Name:      "etc-iscsi-volume",
                                MountPath: "/etc/iscsi",
                                ReadOnly:  true,
                        },
                        {
                                Name:      "etc-machine-id",
                                MountPath: "/etc/machine-id",
                                ReadOnly:  true,
                        },
                        {
                                Name:      "lib-modules-volume",
                                MountPath: "/lib/modules",
                                ReadOnly:  true,
                        },
                        {
                                Name:      "dev-volume",
                                MountPath: "/dev",
                        },
                        {
                                Name:      "run-volume",
                                MountPath: "/run",
                        },
                        {
                                Name:      "sys-volume",
                                MountPath: "/sys",
                        },
                        {
                                Name:      "var-lib-iscsi-volume",
                                MountPath: "/var/lib/iscsi",
                        },
                },
        }
        daemonSet.Spec.Template.Spec.Containers = append(daemonSet.Spec.Template.Spec.Containers, iscsiContainerSpec)

        volConfigs := []corev1.Volume{
                {
                        Name: "etc-machine-id",
                        VolumeSource: corev1.VolumeSource{
                                HostPath: &corev1.HostPathVolumeSource{
                                        Path: "/etc/machine-id",
                                },
                        },
                },
                {
                        Name: "etc-iscsi-volume",
                        VolumeSource: corev1.VolumeSource{
                                HostPath: &corev1.HostPathVolumeSource{
                                        Path: "/etc/iscsi",
                                },
                        },
                },
                {
                        Name: "run-volume",
                        VolumeSource: corev1.VolumeSource{
                                HostPath: &corev1.HostPathVolumeSource{
                                        Path: "/run",
                                },
                        },
                },
                {
                        Name: "dev-volume",
                        VolumeSource: corev1.VolumeSource{
                                HostPath: &corev1.HostPathVolumeSource{
                                        Path: "/dev",
                                },
                        },
                },
                {
                        Name: "sys-volume",
                        VolumeSource: corev1.VolumeSource{
                                HostPath: &corev1.HostPathVolumeSource{
                                        Path: "/sys",
                                },
                        },
                },
                {
                        Name: "lib-modules-volume",
                        VolumeSource: corev1.VolumeSource{
                                HostPath: &corev1.HostPathVolumeSource{
                                        Path: "/lib/modules",
                                },
                        },
                },
                {
                        Name: "var-lib-iscsi-volume",
                        VolumeSource: corev1.VolumeSource{
                                HostPath: &corev1.HostPathVolumeSource{
                                        Path: "/var/lib/iscsi",
                                },
                        },
                },
        }
        for _, volConfig := range volConfigs {
                daemonSet.Spec.Template.Spec.Volumes = append(daemonSet.Spec.Template.Spec.Volumes, volConfig)
        }

        return &daemonSet
}

