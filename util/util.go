package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/crossplane/provider-rancher/apis/rancher/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateKubeconfigSecret(ctx context.Context, kubeconfig []byte, clusterName string, namespace string, kubeClient client.Client) error {
	if namespace == "" {
		namespace = "default"
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-kubeconfig", clusterName),
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"kubeconfig": kubeconfig,
		},
	}
	err := kubeClient.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, &corev1.Secret{})
	if err != nil {
		return kubeClient.Create(ctx, secret)
	} else {
		return kubeClient.Update(ctx, secret)
	}
}

func GenerateKubeconfig(host, clusterID, token, crName, crNamespace string, httpClient http.Client, client client.Client) error {
	ctx := context.Background()
	url := fmt.Sprintf("%s/v3/clusters/%s?action=generateKubeconfig", host, clusterID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
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
	return nil
}

func GetClusters(host, token string, httpClient http.Client) (v1alpha1.ClusterResponse, error) {
	url := fmt.Sprintf("%s/v3/clusters", host)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return v1alpha1.ClusterResponse{}, err
	}
	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return v1alpha1.ClusterResponse{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
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

func CreateCluster(host, token string, httpClient http.Client, cluster *v1alpha1.Cluster) (string, error) {
	url := fmt.Sprintf("%s/v3/clusters", host)
	clusterJson, err := json.Marshal(cluster.Spec.ForProvider.RKE)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(clusterJson))
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
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

func CreateNodePool(host, token string, httpClient http.Client, nodePool *v1alpha1.RKENodePool, clusterId string) error {
	nodePool.ClusterID = clusterId
	url := fmt.Sprintf("%s/v3/nodepool", host)
	nodePoolJson, err := json.Marshal(nodePool)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(nodePoolJson))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		return fmt.Errorf("failed to create node pool: %s", string(body))
	}
	return nil
}

func DeleteCluster(host, token, clusterID string, httpClient http.Client) error {
	url := fmt.Sprintf("%s/v3/clusters/%s", host, clusterID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", token)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to delete cluster: %s", string(body))
	}
	return nil
}
