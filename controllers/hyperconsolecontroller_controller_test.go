package controllers

import (
	"context"
	routev1 "github.com/openshift/api/route/v1"
	"hyper-console-controller/utills"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestHyperConsoleControllerReconciler_CreateHttpsService(t *testing.T) {
	type fields struct {
		Client client.Client
		Scheme *runtime.Scheme
	}
	type args struct {
		ctx        context.Context
		hostedname string
		httpsport  int32
	}
	var objs []client.Object
	objs = append(objs, utills.GetHttpsService("dolin-kubevirt", 32253))
	tests := []struct {
		name     string
		fields   fields
		args     args
		want     error
		wantBool bool
	}{
		{
			name: "the service exist",
			fields: fields{
				Client: fake.NewClientBuilder().WithObjects(objs...).Build(),
			},
			args: args{
				ctx:        context.Background(),
				hostedname: "dolin-kubevirt",
				httpsport:  32253,
			},
			want:     nil,
			wantBool: true,
		},
		{
			name: "the nodeport not currect",
			fields: fields{
				Client: fake.NewClientBuilder().WithObjects(objs...).Build(),
			},
			args: args{
				ctx:        context.Background(),
				hostedname: "dolin-kubevirt",
				httpsport:  32254,
			},
			want:     nil,
			wantBool: false,
		},
		{
			name: "the service not exist",
			fields: fields{
				Client: fake.NewClientBuilder().WithObjects(objs...).Build(),
			},
			args: args{
				ctx:        context.Background(),
				hostedname: "laber-kubevirt",
				httpsport:  32254,
			},
			want:     nil,
			wantBool: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &HyperConsoleControllerReconciler{
				Client: tt.fields.Client,
				Scheme: tt.fields.Scheme,
			}
			got, got1 := r.CreateHttpsService(tt.args.ctx, tt.args.hostedname, tt.args.httpsport)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateHttpsService() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.wantBool {
				t.Errorf("CreateHttpsService() got1 = %v, want %v", got1, tt.wantBool)
			}
		})
	}
}

func TestHyperConsoleControllerReconciler_CreateHttpsRoute(t *testing.T) {
	type fields struct {
		Client client.Client
		Scheme *runtime.Scheme
	}
	type args struct {
		ctx        context.Context
		hostedname string
		basedomain string
	}
	var objs []client.Object
	objs = append(objs, utills.GetRouteForCluster("dolin-kubevirt", "apps.os-gofra.os-pub.com"))
	if err := routev1.AddToScheme(clientgoscheme.Scheme); err != nil {
		t.Fatalf("Unable to add route scheme: (%v)", err)
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		want     error
		wantBool bool
	}{
		{
			name: "the route not exist",
			fields: fields{
				Client: fake.NewClientBuilder().WithObjects(objs...).Build(),
			},
			args: args{
				ctx:        context.Background(),
				hostedname: "laber-kubevirt",
				basedomain: "apps.os-gofra.os-pub.com",
			},
			want:     nil,
			wantBool: false,
		},
		{
			name: "the route is exist",
			fields: fields{
				Client: fake.NewClientBuilder().WithObjects(objs...).Build(),
			},
			args: args{
				ctx:        context.Background(),
				hostedname: "dolin-kubevirt",
				basedomain: "apps.os-gofra.os-pub.com",
			},
			want:     nil,
			wantBool: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &HyperConsoleControllerReconciler{
				Client: tt.fields.Client,
				Scheme: tt.fields.Scheme,
			}
			got, got1 := r.CreateHttpsRoute(tt.args.ctx, tt.args.hostedname, tt.args.basedomain)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateHttpsRoute() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.wantBool {
				t.Errorf("CreateHttpsRoute() got1 = %v, want %v", got1, tt.wantBool)
			}
		})
	}
}

func TestHyperConsoleControllerReconciler_GetHostedKubeConfig(t *testing.T) {
	type fields struct {
		Client client.Client
		Scheme *runtime.Scheme
	}
	type args struct {
		ctx        context.Context
		hostedname string
	}
	var objs []client.Object
	data := []byte{102, 97, 108, 99, 111, 110}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "admin-kubeconfig",
			Namespace: "clusters-dolin-kubevirt",
		},
		Data: map[string][]byte{
			"kubeconfig": data,
		},
	}
	objs = append(objs, &secret)
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr error
	}{
		{
			name: "the get the secret data",
			fields: fields{
				Client: fake.NewClientBuilder().WithObjects(objs...).Build(),
			},
			args: args{
				ctx:        context.Background(),
				hostedname: "dolin-kubevirt",
			},
			want:    []byte{102, 97, 108, 99, 111, 110},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &HyperConsoleControllerReconciler{
				Client: tt.fields.Client,
				Scheme: tt.fields.Scheme,
			}
			got, err := r.GetHostedKubeConfig(tt.args.ctx, tt.args.hostedname)
			if err != tt.wantErr {
				t.Errorf("GetHostedKubeConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetHostedKubeConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}

/*
func TestHyperConsoleControllerReconciler_GetHostedRestKubeConfig(t *testing.T) {
	type fields struct {
		Client client.Client
		Scheme *runtime.Scheme
	}
	type args struct {
		ctx        context.Context
		hostedname string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *rest.Config
		wantErr bool
	}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &HyperConsoleControllerReconciler{
				Client: tt.fields.Client,
				Scheme: tt.fields.Scheme,
			}
			got, err := r.GetHostedRestKubeConfig(tt.args.ctx, tt.args.hostedname)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetHostedRestKubeConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetHostedRestKubeConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}


func TestGetHttpsNodePort(t *testing.T) {
	type args struct {
		config *rest.Config
		ctx    context.Context
	}
	tests := []struct {
		name    string
		args    args
		want    int32
		wantErr bool
	}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetHttpsNodePort(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetHttpsNodePort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetHttpsNodePort() got = %v, want %v", got, tt.want)
			}
		})
	}
}


func TestHyperConsoleControllerReconciler_Reconcile(t *testing.T) {
	type fields struct {
		Client client.Client
		Scheme *runtime.Scheme
	}
	type args struct {
		ctx context.Context
		req controllerruntime.Request
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    controllerruntime.Result
		wantErr bool
	}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &HyperConsoleControllerReconciler{
				Client: tt.fields.Client,
				Scheme: tt.fields.Scheme,
			}
			got, err := r.Reconcile(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Reconcile() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHyperConsoleControllerReconciler_SetupWithManager(t *testing.T) {
	type fields struct {
		Client client.Client
		Scheme *runtime.Scheme
	}
	type args struct {
		mgr controllerruntime.Manager
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &HyperConsoleControllerReconciler{
				Client: tt.fields.Client,
				Scheme: tt.fields.Scheme,
			}
			if err := r.SetupWithManager(tt.args.mgr); (err != nil) != tt.wantErr {
				t.Errorf("SetupWithManager() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

*/
