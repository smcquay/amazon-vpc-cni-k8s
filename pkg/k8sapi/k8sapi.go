// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package k8sapi

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"k8s.io/kubernetes/pkg/api"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
)

const (
	// The kubelet has an internal HTTP server. It serves a read-only view at port 10255.
	// It can return a list of running pods at /pods
	kubeletURL       = "http://localhost:10255/pods"
	kubeletURLPrefix = "http://"
	kubeletURLSurfix = ":10255/pods"
)

// K8SAPIs defines interface to use kubelet introspection API
type K8SAPIs interface {
	K8SGetLocalPodIPs(localIP string) ([]*K8SPodInfo, error)
}

// K8SPodInfo provides pod info
type K8SPodInfo struct {
	// Name is pod's name
	Name string
	// Namespace is pod's namespace
	Namespace string
	// Container is pod's container id
	Container string
	// IP is pod's ipv4 address
	IP string
}

// client provides k8sapi client
type client struct {
}

// ObjectMeta contains json definition
type ObjectMeta struct {
	// Name is pod's name
	Name string `json:"name"`

	// Namespace is pod's namespace
	Namespace string `json:"namespace"`
}

// Pod contains Pod json definition TODO find out why api.PodList can not return Name/Namespace
type Pod struct {
	// Metadata is pod's metadata
	Metadata ObjectMeta `json:"metadata"`

	// Status represents the current information about a pod. This data may not be up
	// to date.
	// +optional
	Status api.PodStatus
}

//TODO find out why api.PodList can not return Name/Namespaces
type podList struct {
	Items []Pod
}

// New returns a client struct
func New() K8SAPIs {
	return &client{}
}

type Getter interface {
	Get(url string) (resp *http.Response, err error)
}

// K8SGetLocalPodIPs queries kubelet about the running Pod information
func (k8s *client) K8SGetLocalPodIPs(localIP string) ([]*K8SPodInfo, error) {
	return k8sGetLocalPodIPs(http.DefaultClient, localIP)
}

func k8sGetLocalPodIPs(http Getter, localIP string) ([]*K8SPodInfo, error) {
	var podsInfo []*K8SPodInfo

	// TODO(tvi): Simplify
	resp, err := http.Get(kubeletURL)
	if err != nil {
		log.Errorf("Failed to query kubelet on localhost: %v", err)
		// retry using node's primary IP
		resp, err = http.Get(kubeletURLPrefix + localIP + kubeletURLSurfix)
		if err != nil {
			log.Errorf("Failed to query kubnet on primary interface %s: %v", localIP, err)
			return podsInfo, errors.Wrap(err, "query kubelet: failed to query kubelet")
		}
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Failed to read kubelet's response body")
		return nil, errors.Wrap(err, "query kublet: failed to read response body")
	}

	var podlist podList
	if err = json.Unmarshal(body, &podlist); err != nil {
		log.Errorf("Failed to unmashal pod list: %v", err)
		return nil, errors.Wrap(err, "query kubelet: failed to unmarshal pod list")
	}

	for idx, pod := range podlist.Items {
		if pod.Status.PodIP != "" {
			podsInfo = append(podsInfo, &K8SPodInfo{
				Name:      pod.Metadata.Name,
				Namespace: pod.Metadata.Namespace,
				IP:        pod.Status.PodIP,
			})

			log.Debugf("Discovered %d podIP %s, name: %s, namespace: %s from kubelet",
				idx, pod.Status.PodIP, pod.Metadata.Name, pod.Metadata.Namespace)
		}
	}

	return podsInfo, nil
}
