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

package main

import (
	"fmt"
	"os"

	"github.com/aws/amazon-vpc-cni-k8s/ipamd"
	log "github.com/cihub/seelog"
)

// const (
// 	defaultLogFilePath = "/host/var/log/aws-routed-eni/ipamd.log"
// )

var (
	Version    = "0.1.3"
	GitVersion = ""
)

func main() {
	defer log.Flush()
	// logger.SetupLogger(logger.GetLogFileLocation(defaultLogFilePath))
	log.Infof("Starting L-IPAMD %v %v  ...", Version, fmt.Sprintf("(%v)", GitVersion))

	ipamd, err := ipamd.New()
	if err != nil {
		log.Error("Could not start L-IPAMD: ", err)
		os.Exit(1)
	}

	go ipamd.StartNodeIPPoolManager()
	go ipamd.SetupHTTP()
	ipamd.RunRPCHandler()
}
