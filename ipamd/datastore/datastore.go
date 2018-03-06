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

import (
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/aws/amazon-vpc-cni-k8s/pkg/k8sapi"
)

const (
	minLifeTime          = 1 * time.Minute
	addressCoolingPeriod = 1 * time.Minute
	// DuplicatedENIError is an error when caller tries to add an duplicate ENI to data store
	DuplicatedENIError = "data store: duplicate ENI"

	// DuplicateIPError is an error when caller tries to add an duplicate IP address to data store
	DuplicateIPError = "datastore: duplicated IP"
)

// ErrUnknownPod is an error when there is no pod in data store matching pod name, namespace, container id
var ErrUnknownPod = errors.New("datastore: unknown pod")

// ErrUnknownPodIP is an error where pod's IP address is not found in data store
var ErrUnknownPodIP = errors.New("datastore: pod using unknown IP address")

// ENIIPPool contains ENI/IP Pool information. Exported fields will be Marshaled for introspection.
type ENIIPPool struct {
	createTime         time.Time
	lastUnAssignedTime time.Time
	// IsPrimary indicates whether ENI is a primary ENI
	IsPrimary bool
	id        string
	// DeviceNumber is the device number of ENI
	DeviceNumber int
	// AssignedIPv4Addresses is the number of IP addesses already been assigned
	AssignedIPv4Addresses int

	// IPv4Addresses shows whether each address is assigned, the key is IP address, which must
	// be in dot-decimal notation with no leading zeros and no whitespace(eg: "10.1.0.253")
	IPv4Addresses map[string]*AddressInfo
}

// AddressInfo contains inforation about an IP, Exported fields will be Marshaled for introspection.
type AddressInfo struct {
	Assigned       bool // true if it is assigned to a pod
	address        string
	unAssignedTime time.Time
}

// PodKey is used to locate pod IP
type PodKey struct {
	name      string
	namespace string
	container string
}

func NewPodKey(k8sPod *k8sapi.K8SPodInfo) PodKey {
	return PodKey{
		name:      k8sPod.Name,
		namespace: k8sPod.Namespace,
		container: k8sPod.Container,
	}
}

func (p PodKey) String() string {
	return fmt.Sprintf("(Name: %s, NameSpace %s Container %s)", p.name, p.namespace, p.container)
}

// PodIPInfo contains pod's IP and the device number of the ENI
type PodIPInfo struct {
	// IP is the IP address of pod
	IP string
	// DeviceNumber is the device number of pod
	DeviceNumber int
}

// DataStore contains node level ENI/IP
type DataStore struct {
	total      int
	assigned   int
	eniIPPools map[string]*ENIIPPool
	podsIP     map[PodKey]PodIPInfo
	lock       sync.RWMutex
}

// PodInfos contains pods IP information which uses key name_namespace_container
type PodInfos map[string]PodIPInfo

// NewDataStore returns DataStore structure
func NewDataStore() *DataStore {
	return &DataStore{
		eniIPPools: make(map[string]*ENIIPPool),
		podsIP:     make(map[PodKey]PodIPInfo),
	}
}

// AddENI add ENI to data store
func (ds *DataStore) AddENI(eniID string, deviceNumber int, isPrimary bool) error {
	ds.lock.Lock()
	defer ds.lock.Unlock()

	log.Debug("datastore: Add an ENI ", eniID)
	if _, ok := ds.eniIPPools[eniID]; ok {
		return errors.New(DuplicatedENIError)
	}

	ds.eniIPPools[eniID] = &ENIIPPool{
		createTime:    time.Now(),
		IsPrimary:     isPrimary,
		id:            eniID,
		DeviceNumber:  deviceNumber,
		IPv4Addresses: make(map[string]*AddressInfo),
	}

	return nil
}

// AddENIIPv4Address add an IP of an ENI to data store
func (ds *DataStore) AddENIIPv4Address(eniID string, ipv4 string) error {
	ds.lock.Lock()
	defer ds.lock.Unlock()

	log.Debugf("Adding ENI(%s)'s IPv4 address %s to datastore", eniID, ipv4)
	log.Debugf("IP Address Pool stats: total: %d, assigned: %d", ds.total, ds.assigned)

	curENI, ok := ds.eniIPPools[eniID]
	if !ok {
		return errors.New("add ENI's IP to datastore: unknown ENI")
	}

	if _, ok := curENI.IPv4Addresses[ipv4]; ok {
		return errors.New(DuplicateIPError)
	}

	ds.total++

	curENI.IPv4Addresses[ipv4] = &AddressInfo{address: ipv4, Assigned: false}
	log.Infof("Added ENI(%s)'s IP %s to datastore", eniID, ipv4)

	return nil
}

func logContainer(k8sPod *k8sapi.K8SPodInfo) string {
	return fmt.Sprintf("(name %s, namespace %s, container %s)", k8sPod.Name, k8sPod.Namespace, k8sPod.Container)
}

// AssignPodIPv4Address assigns an IPv4 address to pod
// It returns the assigned IPv4 address, device number, error
func (ds *DataStore) AssignPodIPv4Address(k8sPod *k8sapi.K8SPodInfo) (string, int, error) {
	ds.lock.Lock()
	defer ds.lock.Unlock()

	log.Debugf("AssignIPv4Address: IP address pool stats: total: %d, assigned: %d", ds.total, ds.assigned)
	podKey := NewPodKey(k8sPod)
	if ipAddr, ok := ds.podsIP[podKey]; ok {
		if ipAddr.IP == k8sPod.IP && k8sPod.IP != "" {
			// The caller invoke multiple times to assign(PodName/NameSpace --> same IPAddress). It is not a error, but not very efficient.
			log.Infof("AssignPodIPv4Address: duplicate pod assign for IP %s, %s", k8sPod.IP, logContainer(k8sPod))
			return ipAddr.IP, ipAddr.DeviceNumber, nil
		}

		//TODO handle this bug assert?, may need to add a counter here, if counter is too high, need to mark node as unhealthy...
		// this is a bug that the caller invoke multiple times to assign(PodName/NameSpace -> a different IPaddress).
		log.Errorf("AssignPodIPv4Address: current IP %s is changed to IP %s for %s", ipAddr, k8sPod.IP, logContainer(k8sPod))
		return "", 0, errors.New("datastore; invalid pod with multiple IP addresses")

	}

	return ds.assignPodIPv4AddressUnsafe(k8sPod)
}

// It returns the assigned IPv4 address, device number, error
func (ds *DataStore) assignPodIPv4AddressUnsafe(k8sPod *k8sapi.K8SPodInfo) (string, int, error) {
	podKey := NewPodKey(k8sPod)
	for _, eni := range ds.eniIPPools {
		if (k8sPod.IP == "") && (len(eni.IPv4Addresses) == eni.AssignedIPv4Addresses) {
			// skip this ENI, since it has no available IP address
			log.Debugf("AssignPodIPv4Address, skip ENI %s that do not have available addresses", eni.id)
			continue
		}
		for _, addr := range eni.IPv4Addresses {
			if k8sPod.IP == addr.address {
				// After L-IPAM restart and built IP warm-pool, it needs to take the existing running pod IP out of the pool.
				if !addr.Assigned {
					ds.assigned++
					eni.AssignedIPv4Addresses++
					addr.Assigned = true
				}
				ds.podsIP[podKey] = PodIPInfo{IP: addr.address, DeviceNumber: eni.DeviceNumber}
				log.Infof("AssignPodIPv4Address Reassign IP %v to pod (name %s, namespace %s)",
					addr.address, k8sPod.Name, k8sPod.Namespace)
				return addr.address, eni.DeviceNumber, nil
			}
			if !addr.Assigned && k8sPod.IP == "" {
				// This is triggered by a pod's Add Network command from CNI plugin
				ds.assigned++
				eni.AssignedIPv4Addresses++
				addr.Assigned = true
				ds.podsIP[podKey] = PodIPInfo{IP: addr.address, DeviceNumber: eni.DeviceNumber}
				log.Infof("AssignPodIPv4Address Assign IP %v to pod %s", addr.address, logContainer(k8sPod))
				return addr.address, eni.DeviceNumber, nil
			}
		}
	}

	log.Infof("DataStore has no available IP addresses")
	return "", 0, errors.New("datastore: no available IP addresses")
}

// GetStats returns statistics
// it returns total number of IP addresses, number of assigned IP addresses
func (ds *DataStore) GetStats() (int, int) {
	return ds.total, ds.assigned
}

func (ds *DataStore) getDeletableENI() *ENIIPPool {
	for _, eni := range ds.eniIPPools {
		if eni.IsPrimary {
			continue
		}

		if time.Now().Sub(eni.createTime) < minLifeTime {
			continue
		}

		if time.Now().Sub(eni.lastUnAssignedTime) < addressCoolingPeriod {
			continue
		}

		if eni.AssignedIPv4Addresses != 0 {
			continue
		}

		log.Debugf("FreeENI: found a deletable ENI %s", eni.id)
		return eni
	}
	return nil
}

// FreeENI free a deletable ENI.
// It returns the name of ENI which is deleted out data store
func (ds *DataStore) FreeENI() (string, error) {
	ds.lock.Lock()
	defer ds.lock.Unlock()

	deletableENI := ds.getDeletableENI()
	if deletableENI == nil {
		log.Debugf("No ENI can be deleted at this time")
		return "", errors.New("free ENI: no ENI can be deleted at this time")
	}

	ds.total -= len(ds.eniIPPools[deletableENI.id].IPv4Addresses)
	ds.assigned -= deletableENI.AssignedIPv4Addresses
	log.Infof("FreeENI %s: IP address pool stats: free %d addresses, total: %d, assigned: %d",
		deletableENI.id, len(ds.eniIPPools[deletableENI.id].IPv4Addresses), ds.total, ds.assigned)

	deletedENI := deletableENI.id
	delete(ds.eniIPPools, deletableENI.id)
	return deletedENI, nil
}

// UnAssignPodIPv4Address
// a) find out the IP address based on PodName and PodNameSpace
// b)  mark IP address as unassigned
// c) returns IP address, ENI's device number, error
func (ds *DataStore) UnAssignPodIPv4Address(k8sPod *k8sapi.K8SPodInfo) (string, int, error) {
	ds.lock.Lock()
	defer ds.lock.Unlock()
	podKey := NewPodKey(k8sPod)
	log.Debugf("UnAssignIPv4Address: IP address pool stats: total:%d, assigned %d, pod %s", ds.total, ds.assigned, podKey.String())

	ipAddr, ok := ds.podsIP[podKey]
	if !ok {
		log.Warnf("UnAssignIPv4Address: Failed to find pod %s", podKey.String())
		return "", 0, ErrUnknownPod
	}

	for _, eni := range ds.eniIPPools {
		if ip, ok := eni.IPv4Addresses[ipAddr.IP]; ok && ip.Assigned {
			ip.Assigned = false
			ds.assigned--
			eni.AssignedIPv4Addresses--

			curTime := time.Now()
			ip.unAssignedTime = curTime
			eni.lastUnAssignedTime = curTime

			log.Infof("UnAssignIPv4Address: pod %v ipAddr %s, DeviceNumber%d", podKey.String(), ip.address, eni.DeviceNumber)
			delete(ds.podsIP, podKey)
			return ip.address, eni.DeviceNumber, nil
		}
	}

	log.Warnf("UnAssignIPv4Address: Failed to find pod %s using IP %s", podKey.String(), ipAddr.IP)
	return "", 0, ErrUnknownPodIP
}

// GetPodInfos provides pod IP information to introspection endpoint
func (ds *DataStore) GetPodInfos() *map[string]PodIPInfo {
	ds.lock.Lock()
	defer ds.lock.Unlock()

	var podInfos = make(map[string]PodIPInfo, len(ds.podsIP))
	for podKey, podInfo := range ds.podsIP {
		key := fmt.Sprintf("%s_%s_%s", podKey.name, podKey.namespace, podKey.container)
		podInfos[key] = podInfo
		log.Debugf("introspect: key %s", key)
	}

	log.Debugf("introspect: len %d", len(ds.podsIP))
	return &podInfos
}

// GetENIInfos provides ENI IP information to introspection endpoint
func (ds *DataStore) GetENIInfos() *ENIInfos {
	ds.lock.Lock()
	defer ds.lock.Unlock()

	var eniInfos = ENIInfos{
		TotalIPs:    ds.total,
		AssignedIPs: ds.assigned,
		ENIIPPools:  make(map[string]ENIIPPool, len(ds.eniIPPools)),
	}

	for eni, eniInfo := range ds.eniIPPools {
		eniInfos.ENIIPPools[eni] = *eniInfo
	}
	return &eniInfos
}

// GetENIs provides the number of ENI in the datastore
func (ds *DataStore) GetENIs() int {
	ds.lock.Lock()
	defer ds.lock.Unlock()
	return len(ds.eniIPPools)
}
