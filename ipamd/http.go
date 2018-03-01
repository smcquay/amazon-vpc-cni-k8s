// Copyright 2014-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//      http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package ipamd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

const (
	httpPort = ":51678"
)

func createDebugHandler(fetcher func() interface{}) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		responseJSON, err := json.Marshal(fetcher())
		if err != nil {
			log.Error("Failed to marshal data: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Write(responseJSON)
	}
}

// SetupHTTP sets up introspection service endpoint and prometheus metrics.
func (c *IPAMD) SetupHTTP() {
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", createDebugHandler(func() interface{} {
		return []string{"/v1/enis", "/v1/pods"}
	}))
	serveMux.HandleFunc("/v1/enis", createDebugHandler(func() interface{} {
		return c.dataStore.GetENIInfos()
	}))
	serveMux.HandleFunc("/v1/pods", createDebugHandler(func() interface{} {
		return c.dataStore.GetPodInfos()
	}))
	handler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "TODO")
	}
	serveMux.HandleFunc("/live", handler)
	serveMux.HandleFunc("/ready", handler)
	serveMux.Handle("/metrics", promhttp.Handler())
	s := &http.Server{
		Addr:           httpPort,
		Handler:        serveMux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}
