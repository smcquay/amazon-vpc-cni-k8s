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

package datastore

import "github.com/aws/amazon-vpc-cni-k8s/pkg/k8sapi"

type DS interface {
	AddENI(eniID string, deviceNumber int, isPrimary bool) error
	AddENIIPv4Address(eniID string, ipv4 string) error

	AssignPodIPv4Address(k8sPod *k8sapi.K8SPodInfo) (string, int, error)
	UnAssignPodIPv4Address(k8sPod *k8sapi.K8SPodInfo) (string, int, error)

	GetStats() (int, int)
	GetENIInfos() *ENIInfos
	GetPodInfos() *map[string]PodIPInfo
	FreeENI() (string, error)
}

// ENIInfos contains ENI IP information
type ENIInfos struct {
	// TotalIPs is the total number of IP addresses
	TotalIPs int
	// assigned is the number of IP addresses that has been assigned
	AssignedIPs int
	// ENIIPPools contains ENI IP pool information
	ENIIPPools map[string]ENIIPPool
}

// PodIPInfo contains pod's IP and the device number of the ENI
type PodIPInfo struct {
	// IP is the IP address of pod
	IP string
	// DeviceNumber is the device number of pod
	DeviceNumber int
}
