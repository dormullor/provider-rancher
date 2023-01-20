package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/dormullor/provider-rancher/apis/rke1/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GenerateKubeconfigSecret(kubeconfig []byte, clusterName string, namespace string) corev1.Secret {
	if namespace == "" {
		namespace = "default"
	}
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-kubeconfig", clusterName),
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"kubeconfig": kubeconfig,
		},
	}
}

func CreateKubeconfigSecret(ctx context.Context, kubeconfig []byte, clusterName string, namespace string, kubeClient client.Client) error {
	secret := GenerateKubeconfigSecret(kubeconfig, clusterName, namespace)
	exist := KubeconfigSecretExist(ctx, clusterName, namespace, kubeClient)
	if !exist {
		return kubeClient.Create(ctx, &secret)
	}
	return kubeClient.Update(ctx, &secret)
}

func KubeconfigSecretExist(ctx context.Context, clusterName string, namespace string, kubeClient client.Client) bool {
	err := kubeClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-kubeconfig", clusterName), Namespace: namespace}, &corev1.Secret{})
	return err == nil
}

func GenerateKubeconfig(ctx context.Context, host, clusterID, token, crName, crNamespace string, httpClient http.Client, client client.Client) error {
	exist := KubeconfigSecretExist(ctx, crName, crNamespace, client)
	if !exist {
		url := fmt.Sprintf("%s/v3/clusters/%s?action=generateKubeconfig", host, clusterID)
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return err
		}
		req.Header.Add("Authorization", token)
		req.Header.Add("Accept", "application/json")
		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer dclose(resp.Body)

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var result *v1alpha1.KubeconfigResponse
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Println("Can not unmarshal JSON")
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("failed to generate kubeconfig: %s", string(body))
		}
		err = CreateKubeconfigSecret(ctx, []byte(result.Config), crName, crNamespace, client)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetClusters(host, token string, httpClient http.Client, ctx context.Context) (v1alpha1.ClusterResponse, error) {
	url := fmt.Sprintf("%s/v3/clusters", host)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return v1alpha1.ClusterResponse{}, err
	}
	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return v1alpha1.ClusterResponse{}, err
	}
	defer dclose(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return v1alpha1.ClusterResponse{}, err
	}

	var result *v1alpha1.ClusterResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Can not unmarshal JSON")
	}
	if resp.StatusCode != 200 {
		return v1alpha1.ClusterResponse{}, fmt.Errorf("failed to get clusters: %s", string(body))
	}

	return *result, nil
}

func CreateCluster(host, token string, httpClient http.Client, cluster *v1alpha1.RKE1Cluster, ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/v3/clusters", host)
	clusterJson, err := json.Marshal(cluster.Spec.ForProvider.RKE)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(clusterJson))
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer dclose(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result *v1alpha1.Data
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Can not unmarshal JSON")
	}
	if resp.StatusCode != 201 {
		return "", fmt.Errorf("failed to create cluster: %s", string(body))
	}

	return result.ID, nil
}

func CreateNodePool(host, token, clusterId string, httpClient http.Client, nodePool v1alpha1.RKENodePool, ctx context.Context) error {
	nodePool.ClusterID = clusterId
	url := fmt.Sprintf("%s/v3/nodepool", host)
	nodePoolJson, err := json.Marshal(nodePool)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(nodePoolJson))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer dclose(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		return fmt.Errorf("failed to create node pool: %s", string(body))
	}
	return nil
}

func DeleteCluster(host, token, clusterID string, httpClient http.Client, ctx context.Context) error {
	url := fmt.Sprintf("%s/v3/clusters/%s", host, clusterID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer dclose(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to delete cluster: %s", string(body))
	}
	return nil
}

func dclose(c io.Closer) {
	if err := c.Close(); err != nil {
		log.Fatal(err)
	}
}

func CreateNodeTemplate(host, token string, httpClient http.Client, nodeTemplate v1alpha1.RKE1NodeTemplate, ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/v3/nodetemplate", host)
	nodeTemplateJson, err := json.Marshal(nodeTemplate.Spec.ForProvider)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(nodeTemplateJson))
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer dclose(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result *v1alpha1.Data
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Can not unmarshal JSON")
	}
	if resp.StatusCode != 201 {
		return "", fmt.Errorf("failed to create node template: %s", string(body))
	}

	return result.ID, nil
}

func DeleteNodeTemplate(host, token, nodeTemplateID string, httpClient http.Client, ctx context.Context) error {
	url := fmt.Sprintf("%s/v3/nodetemplates/%s", host, nodeTemplateID)
	fmt.Println(url)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer dclose(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to delete node template: %s", string(body))
	}
	return nil
}

func GetNodeTemplates(host, token string, httpClient http.Client, ctx context.Context) (v1alpha1.ClusterResponse, error) {
	url := fmt.Sprintf("%s/v3/nodetemplates", host)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return v1alpha1.ClusterResponse{}, err
	}
	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return v1alpha1.ClusterResponse{}, err
	}
	defer dclose(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return v1alpha1.ClusterResponse{}, err
	}

	var result *v1alpha1.ClusterResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Can not unmarshal JSON")
	}
	if resp.StatusCode != 200 {
		return v1alpha1.ClusterResponse{}, fmt.Errorf("failed to get node templates: %s", string(body))
	}

	return *result, nil
}

func GetVpcIdByTags(tags map[string]string, region string, Credentials *credentials.Credentials) (string, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: Credentials,
	},
	)
	if err != nil {
		return "", err
	}
	svc := ec2.New(sess)
	input := &ec2.DescribeVpcsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []*string{aws.String(tags["Name"])},
			},
			{
				Name:   aws.String("tag:ManagedBy"),
				Values: []*string{aws.String(tags["ManagedBy"])},
			},
		},
	}
	result, err := svc.DescribeVpcs(input)
	if err != nil {
		return "", err
	}
	if len(result.Vpcs) == 0 {
		return "", fmt.Errorf("vpc not found")
	}
	return *result.Vpcs[0].VpcId, nil
}

func GetSubnetIdByTags(tags map[string]string, region string, Credentials *credentials.Credentials) (string, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: Credentials},
	)
	if err != nil {
		return "", err
	}
	svc := ec2.New(sess)
	input := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []*string{aws.String(tags["Name"])},
			},
			{
				Name:   aws.String("tag:ManagedBy"),
				Values: []*string{aws.String(tags["ManagedBy"])},
			},
		},
	}
	result, err := svc.DescribeSubnets(input)
	if err != nil {
		return "", err
	}
	if len(result.Subnets) == 0 {
		return "", fmt.Errorf("subnet not found")
	}
	return *result.Subnets[0].SubnetId, nil
}

func GetNodeTemplateByName(host, token, name string, httpClient http.Client, ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/v3/nodetemplates?name=%s", host, name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer dclose(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result *v1alpha1.ClusterResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Can not unmarshal JSON")
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to get node template: %s", string(body))
	}

	return result.Data[0].ID, nil
}
