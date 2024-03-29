/*
Copyright 2022 The Crossplane Authors.

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

package rke1cluster

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/dormullor/provider-rancher/apis/rke1/v1alpha1"
	apisv1alpha1 "github.com/dormullor/provider-rancher/apis/v1alpha1"
	"github.com/dormullor/provider-rancher/internal/controller/features"
	"github.com/dormullor/provider-rancher/util"
)

const (
	errNotCluster   = "managed resource is not a Cluster custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"
)

// Setup adds a controller that reconciles Cluster managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ClusterGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ClusterGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{})}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.RKE1Cluster{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube  client.Client
	usage resource.Tracker
}

// Connect typically produces an ExternalClient
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.RKE1Cluster)
	if !ok {
		return nil, errors.New(errNotCluster)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	cd := pc.Spec.Credentials
	tokenDecoded, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}
	rancherHost := pc.Spec.RancherHost
	token := "Basic " + b64.StdEncoding.EncodeToString(tokenDecoded)
	client := &http.Client{}
	return &external{httpClient: *client, token: token, kube: c.kube, rancherHost: rancherHost}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	httpClient  http.Client
	token       string
	rancherHost string
	kube        client.Client
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.RKE1Cluster)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotCluster)
	}

	results, err := util.GetClusters(c.rancherHost, c.token, c.httpClient, ctx)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	clusterFound := false
	for _, cluster := range results.Data {
		if cluster.Name == cr.Name {
			clusterFound = true
			cr.Status.AtProvider.ID = cluster.ID
			if cluster.State == "active" {
				cr.Status.SetConditions(xpv1.Available())
				err := util.GenerateKubeconfig(ctx, c.rancherHost, cluster.ID, c.token, cr.Name, cr.Spec.ForProvider.KubeconfigSecretNamespace, c.httpClient, c.kube)
				if err != nil {
					return managed.ExternalObservation{}, err
				}
			} else {
				cr.Status.SetConditions(xpv1.Unavailable())
			}
		}
	}

	return managed.ExternalObservation{
		ResourceExists:    clusterFound,
		ResourceUpToDate:  true,
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.RKE1Cluster)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotCluster)
	}

	for index, node := range cr.Spec.ForProvider.NodePools {
		if node.NodeTemplateIDRef != "" {
			nodeTemplateID, err := util.GetNodeTemplateByName(c.rancherHost, c.token, node.NodeTemplateIDRef, c.httpClient, ctx)
			if err != nil {
				return managed.ExternalCreation{}, err
			}
			cr.Spec.ForProvider.NodePools[index].NodeTemplateID = nodeTemplateID
		}
	}

	clusterId, err := util.CreateCluster(c.rancherHost, c.token, c.httpClient, cr, ctx)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	for _, node := range cr.Spec.ForProvider.NodePools {
		err := util.CreateNodePool(c.rancherHost, c.token, clusterId, c.httpClient, node, ctx)
		if err != nil {
			return managed.ExternalCreation{}, err
		}
	}
	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.RKE1Cluster)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotCluster)
	}

	fmt.Printf("Updating: %+v", cr)

	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.RKE1Cluster)
	if !ok {
		return errors.New(errNotCluster)
	}
	err := util.DeleteCluster(c.rancherHost, c.token, cr.Status.AtProvider.ID, c.httpClient, ctx)
	if err != nil {
		return err
	}
	return nil
}
