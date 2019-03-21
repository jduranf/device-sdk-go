// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
//
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"fmt"

	"github.com/edgexfoundry/device-sdk-go/internal/common"
)

func DiscoveryHandler(requestMap map[string]string) {

	var mods [8][2]string

	modules, err := common.Driver.Discover()
	if err != nil {
		return
	}
	//TODO: Handle discovery results: Read config Provision Watcher, compare identifiers
	//and create new devices if was necessary

	//Example Handle discovery results: Print results
	mods = modules.([8][2]string)
	for i := range mods {
		switch mods[i][0] {
		case " Err", "":
			fmt.Printf("Error in Module %d \n", i+1)
		case "IlAd":
			fmt.Printf("Module %d responds illegal data address exception \n", i+1)
		case "Empt":
			fmt.Printf("Module %d is empty \n", i+1)
		case "CVMD":
			fmt.Printf("Module %d with Serial %s is a CVMD \n", i+1, mods[i][1])
		case "TDIO":
			fmt.Printf("Module %d with Serial %s is a TDIO \n", i+1, mods[i][1])
		case "RDIO":
			fmt.Printf("Module %d with Serial %s is a RDIO \n", i+1, mods[i][1])
		case "VDIO":
			fmt.Printf("Module %d with Serial %s is a VDIO \n", i+1, mods[i][1])
		case "MAIO":
			fmt.Printf("Module %d with Serial %s is a MAIO \n", i+1, mods[i][1])
		case "CAIO":
			fmt.Printf("Module %d with Serial %s is a CAIO \n", i+1, mods[i][1])
		default:
			fmt.Printf("Module %s with Serial %s \n", mods[i][0], mods[i][1])
		}
	}
	common.LoggingClient.Info(fmt.Sprintf("service: discovery request"))
}

func TransformHandler(requestMap map[string]string) (map[string]string, common.AppError) {
	common.LoggingClient.Info(fmt.Sprintf("service: transform request: transformData: %s", requestMap["transformData"]))
	return requestMap, nil
}
