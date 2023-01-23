package v1alpha1

type ClusterResponse struct {
	Data []Data `json:"data"`
}

type Data struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Name  string `json:"name"`
}
