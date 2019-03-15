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

	modules, err := common.Driver.Discover()
	if err != nil {
	}

	//TODO: Handle modules
	mods, _ := modules.([8]string)
	for i := range mods {
		switch mods[i] {
		case "Error", "":
			fmt.Printf("Error in Module %d", i)
			fmt.Println("")
		case "Empty":
			fmt.Printf("Module %d is empty", i)
			fmt.Println("")
		case "EDS0":
			fmt.Printf("Module %d is an EDS0", i)
			fmt.Println("")
		default:
			fmt.Println("Default")
		}
	}

	common.LoggingClient.Info(fmt.Sprintf("service: discovery request"))
}

func TransformHandler(requestMap map[string]string) (map[string]string, common.AppError) {
	common.LoggingClient.Info(fmt.Sprintf("service: transform request: transformData: %s", requestMap["transformData"]))
	return requestMap, nil
}
