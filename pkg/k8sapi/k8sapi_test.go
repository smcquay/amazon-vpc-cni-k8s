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
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	"k8s.io/kubernetes/pkg/api"

	"github.com/aws/amazon-vpc-cni-k8s/pkg/httpwrapper/mocks"
	"github.com/aws/amazon-vpc-cni-k8s/pkg/ioutilwrapper/mocks"
)

const (
	pod1IP = "10.0.10.10"
	pod2IP = "10.0.20.20"
	testIP = "10.10.0.1"
)

func setup(t *testing.T) (*gomock.Controller,
	*mock_ioutilwrapper.MockIOUtil,
	*mock_httpwrapper.MockHTTP) {
	ctrl := gomock.NewController(t)
	return ctrl,
		mock_ioutilwrapper.NewMockIOUtil(ctrl),
		mock_httpwrapper.NewMockHTTP(ctrl)
}

type mockHTTPResp struct{}

func (*mockHTTPResp) Read([]byte) (int, error) {
	return 0, nil
}

func (*mockHTTPResp) Close() error {
	return nil
}

func NewmockHTTPResp() io.ReadCloser {
	return &mockHTTPResp{}
}

type getter struct {
	fn func(url string) (resp *http.Response, err error)
}

func (g getter) Get(url string) (resp *http.Response, err error) {
	return g.fn(url)
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func TestK8SGetLocalPodIPs(t *testing.T) {
	pod1 := api.Pod{Status: api.PodStatus{PodIP: pod1IP}}
	pod2 := api.Pod{Status: api.PodStatus{PodIP: pod2IP}}
	testResp := &api.PodList{Items: []api.Pod{pod1, pod2}}
	testRespByte, _ := json.Marshal(testResp)

	podsInfo, err := k8sGetLocalPodIPs(getter{func(url string) (resp *http.Response, err error) {
		if url == kubeletURL {
			return &http.Response{
				Body: nopCloser{bytes.NewBuffer(testRespByte)},
			}, nil
		}
		return nil, nil
	}}, testIP)

	_, _ = podsInfo, err

	// TODO (tvi): reenable
	// assert.NoError(t, err)
	// assert.Equal(t, len(podsInfo), 2)
}
