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

package rke1nodetemplate

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"net/http"
	"reflect"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/aws/aws-sdk-go/aws/credentials"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/dormullor/provider-rancher/apis/rke1/v1alpha1"
	apisv1alpha1 "github.com/dormullor/provider-rancher/apis/v1alpha1"
	"github.com/dormullor/provider-rancher/internal/controller/features"
	"github.com/dormullor/provider-rancher/util"
)

const (
	errNotRKE1NodeTemplate    = "managed resource is not a RKE1NodeTemplate custom resource"
	errTrackPCUsage           = "cannot track ProviderConfig usage"
	errGetPC                  = "cannot get ProviderConfig"
	errGetCreds               = "cannot get credentials"
	errCreateRKE1NodeTemplate = "cannot create RKE1NodeTemplate"
	ManagedByCrossplane       = "crossplane"
)

// Setup adds a controller that reconciles RKE1NodeTemplate managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.RKE1NodeTemplateGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.RKE1NodeTemplateGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{})}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.RKE1NodeTemplate{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube  client.Client
	usage resource.Tracker
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.RKE1NodeTemplate)
	if !ok {
		return nil, errors.New(errNotRKE1NodeTemplate)
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

	awsCredentials := &credentials.Credentials{}
	if !reflect.DeepEqual(pc.Spec.AWScreds, apisv1alpha1.AWScreds{}) {
		ak := pc.Spec.AWScreds.AccessKeyID
		sk := pc.Spec.AWScreds.SecretAccessKey
		awsAccessKey, err := resource.CommonCredentialExtractor(ctx, ak.Source, c.kube, ak.CommonCredentialSelectors)
		if err != nil {
			return nil, errors.Wrap(err, errGetCreds)
		}
		awsSecretKey, err := resource.CommonCredentialExtractor(ctx, sk.Source, c.kube, sk.CommonCredentialSelectors)
		if err != nil {
			return nil, errors.Wrap(err, errGetCreds)
		}
		awsCredentials = credentials.NewStaticCredentials(string(awsAccessKey), string(awsSecretKey), "")
	}

	rancherHost := pc.Spec.RancherHost
	token := "Basic " + b64.StdEncoding.EncodeToString(tokenDecoded)
	client := &http.Client{}
	return &external{
		httpClient:     *client,
		token:          token,
		kube:           c.kube,
		rancherHost:    rancherHost,
		awsCredentials: awsCredentials,
	}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	httpClient     http.Client
	token          string
	rancherHost    string
	kube           client.Client
	awsCredentials *credentials.Credentials
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.RKE1NodeTemplate)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotRKE1NodeTemplate)
	}

	results, err := util.GetNodeTemplates(c.rancherHost, c.token, c.httpClient, ctx)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	templateFound := false
	for _, template := range results.Data {
		if template.Name == cr.Name {
			templateFound = true
			cr.Status.AtProvider.ID = template.ID
			if template.State == "active" {
				cr.Status.SetConditions(xpv1.Available())
			} else {
				cr.Status.SetConditions(xpv1.Unavailable())
			}
		}
	}

	return managed.ExternalObservation{
		ResourceExists:    templateFound,
		ResourceUpToDate:  true,
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.RKE1NodeTemplate)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotRKE1NodeTemplate)
	}

	if cr.Spec.ForProvider.Amazonec2Config.VpcIDRef != "" {
		tags := map[string]string{
			"Name":      cr.Spec.ForProvider.Amazonec2Config.VpcIDRef,
			"ManagedBy": ManagedByCrossplane,
		}
		vpcID, err := util.GetVpcIdByTags(tags, cr.Spec.ForProvider.Amazonec2Config.Region, c.awsCredentials)
		if err != nil {
			return managed.ExternalCreation{}, err
		}
		cr.Spec.ForProvider.Amazonec2Config.VpcID = vpcID
	}

	if cr.Spec.ForProvider.Amazonec2Config.SubnetIDRef != "" {
		tags := map[string]string{
			"Name":      cr.Spec.ForProvider.Amazonec2Config.SubnetIDRef,
			"ManagedBy": ManagedByCrossplane,
		}
		subnetID, err := util.GetSubnetIdByTags(tags, cr.Spec.ForProvider.Amazonec2Config.Region, c.awsCredentials)
		if err != nil {
			return managed.ExternalCreation{}, err
		}
		cr.Spec.ForProvider.Amazonec2Config.SubnetID = subnetID
	}

	_, err := util.CreateNodeTemplate(c.rancherHost, c.token, c.httpClient, *cr, ctx)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateRKE1NodeTemplate)
	}

	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.RKE1NodeTemplate)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotRKE1NodeTemplate)
	}

	fmt.Printf("Updating: %+v", cr)

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.RKE1NodeTemplate)
	if !ok {
		return errors.New(errNotRKE1NodeTemplate)
	}
	err := util.DeleteNodeTemplate(c.rancherHost, c.token, cr.Status.AtProvider.ID, c.httpClient, ctx)
	if err != nil {
		return err
	}
	return nil
}
