package utills

import (
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

//GetHttpsService creates a Service object for the infra cluster
func GetHttpsService(clustername string, nodeport int32) *corev1.Service {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "apps-ingress",
			Namespace:   "clusters-" + clustername,
			Annotations: map[string]string{"dana.io/console": "True"},
			//			Labels:      map[string]string{"app": clustername, "dolin": "test"},
		},
		Spec: corev1.ServiceSpec{
			IPFamilies: []v1.IPFamily{"IPv4"},
			Ports: []corev1.ServicePort{{ //TODO currently I assume there will be only one port for each service, I dont know if its a use-case but probably should handle this
				Name:       "https-443",
				Protocol:   "TCP",
				Port:       443,
				TargetPort: intstr.IntOrString{IntVal: nodeport},
			}},
			Selector: map[string]string{"kubevirt.io": "virt-launcher"},
			Type:     "ClusterIP",
		},
	}
	return &service
}

//GetHttpsRoute creates a Route object for the infra cluster
func GetRouteForCluster(clustername string, basedomain string) *routev1.Route {
	var numWeight int32 = 100
	route := routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:        clustername + "-443",
			Namespace:   "clusters-" + clustername,
			Annotations: map[string]string{"dana.io/console": "True"},
			//Labels:    map[string]string{"dolin": "test"},
		},
		Spec: routev1.RouteSpec{
			Host:           "data.apps." + clustername + "." + basedomain,
			WildcardPolicy: "Subdomain",
			TLS: &routev1.TLSConfig{
				Termination: "passthrough",
			},
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   "apps-ingress",
				Weight: &numWeight,
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("https-443"),
			},
		},
	}
	return &route
}
