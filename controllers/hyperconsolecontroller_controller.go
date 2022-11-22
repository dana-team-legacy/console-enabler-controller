/*
Copyright 2022.

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
	"console-enabler-controller/utills"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var clientset *kubernetes.Clientset

// HyperConsoleControllerReconciler reconciles a HyperConsoleController object
type HyperConsoleControllerReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	ResourcePoolEvents chan event.GenericEvent
}

type NamespacePredicate struct {
	predicate.Funcs
}

//+kubebuilder:rbac:groups=console-enabler-controller.dana.io,resources=hyperconsolecontrollers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=console-enabler-controller.dana.io,resources=hyperconsolecontrollers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=console-enabler-controller.dana.io,resources=hyperconsolecontrollers/finalizers,verbs=update
// +kubebuilder:rbac:groups=console-enabler-controller.dana.io,resources=deployment,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;patch;update;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;patch;update;watch
//+kubebuilder:rbac:groups="",resources=routes,verbs=get;list;patch;update;watch
//+kubebuilder:rbac:groups="hypershift.openshift.io",resources=HostedCluster,verbs=get;list;patch;update;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HyperConsoleController object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *HyperConsoleControllerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx, "name", req.Name)
	l.Info("starting to Reconcile")
	hostedclusterres := unstructured.Unstructured{}
	hostedclusterres.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "hypershift.openshift.io",
		Version: "v1alpha1",
		Kind:    "HostedCluster",
	})
	//Get the hostedcluster
	//if err := r.Client.Get(ctx, types.NamespacedName{Namespace: "clusters"}, &hostedclusterres); err != nil {
	if err := r.Client.Get(ctx, req.NamespacedName, &hostedclusterres); err != nil {
		if errors.IsNotFound(err) {
			l.Info("hostedclusters not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, nil
	}
	// Check if the cluster is ready and available
	conditions, _, err := unstructured.NestedSlice(hostedclusterres.Object, "status", "conditions")
	isAvailable := FoundStatusAvailable(conditions)
	if !isAvailable {
		l.Info("The hostedcluster is not available yet")
		return ctrl.Result{}, nil
	}
	// This is good i need to search how to take the available status
	//Get the kubeconfig from the hostedcluster namespace
	config, err := r.GetHostedRestKubeConfig(ctx, hostedclusterres.GetName())
	if err != nil {
		if errors.IsNotFound(err) {
			l.Info("secrets not found")
			return ctrl.Result{}, nil
		}
	}
	// Get the nodeport from the hostedcluster
	httpsport, err := GetHttpsNodePort(ctx, config)
	if err != nil {
		l.Info("nodePort not found")
		return ctrl.Result{}, err
	}
	// Here I create a service if it not exist
	if err, flag := r.CreateHttpsService(ctx, hostedclusterres.GetName(), httpsport); err != nil && flag {
		l.Info("The service not create get error")
		return ctrl.Result{}, err
	} else if err == nil && !flag {
		l.Info("The service is create/Update")
	}
	// Here I create a Route if it not exist
	basedomain, _, err := unstructured.NestedStringMap(hostedclusterres.Object, "spec", "dns")
	if err, flag := r.CreateHttpsRoute(ctx, hostedclusterres.GetName(), basedomain["baseDomain"]); err != nil && flag {
		l.Info("The Route not create get error")
		return ctrl.Result{}, err
	} else if err == nil && !flag {
		l.Info("The Route is create")
	}

	return ctrl.Result{}, nil
}

func FoundStatusAvailable(status []interface{}) bool {
	for _, value := range status {
		con := mapToCondition(value)
		if con.Type == "Available" {
			if con.Status == "True" {
				return true
			}
			return false
		}
	}
	return false
}

func mapToCondition(m interface{}) metav1.Condition {
	// Convert map to json string
	jsonStr, err := json.Marshal(m)
	if err != nil {
		fmt.Println(err)
	}

	// Convert json string to struct
	var condition metav1.Condition
	if err := json.Unmarshal(jsonStr, &condition); err != nil {
		fmt.Println(err)
	}
	return condition
}

// CreateHttpsRoute get hostedcluster name, client and basecomain
//  Check if the route exist and if not create a new https route for the hostedcluster
func (r *HyperConsoleControllerReconciler) CreateHttpsRoute(ctx context.Context, hostedname string, basedomain string) (error, bool) {
	namespace := "clusters-" + hostedname
	httpsRoute := routev1.Route{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: hostedname + "-443", Namespace: namespace}, &httpsRoute); err != nil {
		// Check if the Route is exist on the hostedcluster namespace
		if errors.IsNotFound(err) {
			hostedRoute := utills.GetRouteForCluster(hostedname, basedomain)
			if err := r.Client.Create(ctx, hostedRoute); err != nil {
				fmt.Printf("The Route get error in the create")
				return err, true
			} else {
				fmt.Printf("The Route is create")
				return err, false
			}
		}
	}
	return nil, true
}

// CreateHttpsService get hostedcluster name, client and httpsnodeport
//  Check if the service exist and if not create a new https service for the hostedcluster
func (r *HyperConsoleControllerReconciler) CreateHttpsService(ctx context.Context, hostedname string, httpsport int32) (error, bool) {
	namespace := "clusters-" + hostedname
	httpsService := corev1.Service{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: "apps-ingress", Namespace: namespace}, &httpsService); err != nil {
		// Check if the service is exist on the hostedcluster namespace
		if errors.IsNotFound(err) {
			hostedSVC := utills.GetHttpsService(hostedname, httpsport)
			if err := r.Client.Create(ctx, hostedSVC); err != nil {
				fmt.Printf("The service not create get error")
				return err, true
			} else {
				fmt.Printf("The service is Create")
				return err, false
			}

		}
		//Check if the nodeport that open on the hosted cluster is the same targetPort in the service
	} else if httpsport != httpsService.Spec.Ports[0].TargetPort.IntVal {
		hostedSVC := utills.GetHttpsService(hostedname, httpsport)
		if err := r.Client.Update(ctx, hostedSVC); err != nil {
			fmt.Printf("The service is not update get error")
			return err, true
		} else {
			fmt.Printf("The service is Update")
			return err, false
		}
	}
	return nil, true
}

// GetHttpsNodePort get kubeconfig
// Take from the hostedcluster the https nodeport
// Return 0 if the nodeport doesn't exist
func GetHttpsNodePort(ctx context.Context, config *rest.Config) (int32, error) {
	hostedclientset := kubernetes.NewForConfigOrDie(config)
	namespace := "openshift-ingress"
	service, err := hostedclientset.CoreV1().Services(namespace).Get(ctx, "router-nodeport-default", metav1.GetOptions{})
	if err != nil {
		return 0, err
	} else {
		for _, port := range service.Spec.Ports {
			if port.Name == "https" {
				return port.NodePort, nil
			}
		}
	}
	return 0, nil
}

// GetHostedKubeConfig get hostname and client and return the kubeconfig from a secret in the hostedcluster namespace
func (r *HyperConsoleControllerReconciler) GetHostedKubeConfig(ctx context.Context, hostedname string) ([]byte, error) {
	kubeconfig := v1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: "admin-kubeconfig", Namespace: "clusters-" + hostedname}, &kubeconfig); err != nil {
		return nil, err
	}
	return kubeconfig.Data["kubeconfig"], nil
}

// GetHostedKubeConfig get hostname and client and return the kubeconfig from a secret in the hostedcluster namespace
func (r *HyperConsoleControllerReconciler) GetHostedRestKubeConfig(ctx context.Context, hostedname string) (*rest.Config, error) {
	hostedconfig, err := r.GetHostedKubeConfig(ctx, hostedname)
	if err != nil {
		return nil, err
	}
	config, err := clientcmd.NewClientConfigFromBytes(hostedconfig)
	if err != nil {
		return nil, err
	}
	return config.ClientConfig()
}

// SetupWithManager sets up the controller with the Manager.
func (r *HyperConsoleControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	hostedclusterres := unstructured.Unstructured{}
	hostedclusterres.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "hypershift.openshift.io",
		Version: "v1alpha1",
		Kind:    "HostedCluster",
	})

	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		Watches(&source.Kind{Type: &corev1.Service{}}, handler.EnqueueRequestsFromMapFunc(r.ServiceFetcher)).
		Watches(&source.Kind{Type: &routev1.Route{}}, handler.EnqueueRequestsFromMapFunc(r.RouteFetcher)).
		WithEventFilter(NamespacePredicate{predicate.NewPredicateFuncs(func(object client.Object) bool {
			if reflect.TypeOf(object) == reflect.TypeOf(&corev1.Service{}) || reflect.TypeOf(object) == reflect.TypeOf(&hostedclusterres) || reflect.TypeOf(object) == reflect.TypeOf(&routev1.Route{}) {
				objAnnotations := object.GetAnnotations()
				if _, ok := objAnnotations["dana.io/console"]; ok {
					return true
				}
				return false
			}
			return false
		})}).
		For(&hostedclusterres).
		Complete(r)
}

func (r *HyperConsoleControllerReconciler) ServiceFetcher(o client.Object) []ctrl.Request {
	var result []ctrl.Request
	service, ok := o.(*corev1.Service)
	if !ok {
		panic(fmt.Sprintf("Exepted a Service but got a %v", o.GetName()))
	}
	hostedclustername := strings.Split(service.GetNamespace(), "clusters-")
	hostedcluster := unstructured.Unstructured{}
	hostedcluster.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "hypershift.openshift.io",
		Version: "v1alpha1",
		Kind:    "HostedCluster",
	})
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Namespace: "clusters", Name: hostedclustername[1]}, &hostedcluster); err != nil {
		return nil
	}
	name := client.ObjectKey{Namespace: "clusters", Name: hostedcluster.GetName()}
	result = append(result, ctrl.Request{NamespacedName: name})
	return result
}

func (r *HyperConsoleControllerReconciler) RouteFetcher(o client.Object) []ctrl.Request {
	var result []ctrl.Request
	route, ok := o.(*routev1.Route)
	if !ok {
		panic(fmt.Sprintf("Exepted a Service but got a %v", o.GetName()))
	}
	hostedclustername := strings.Split(route.GetNamespace(), "clusters-")
	hostedcluster := unstructured.Unstructured{}
	hostedcluster.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "hypershift.openshift.io",
		Version: "v1alpha1",
		Kind:    "HostedCluster",
	})
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Namespace: "clusters", Name: hostedclustername[1]}, &hostedcluster); err != nil {
		return nil
	}
	name := client.ObjectKey{Namespace: "clusters", Name: hostedcluster.GetName()}
	result = append(result, ctrl.Request{NamespacedName: name})
	return result
}
