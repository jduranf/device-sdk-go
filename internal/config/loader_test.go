// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
// Copyright (c) 2019 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
)

func TestLoadDriverConfigFromFile(t *testing.T) {
	expectedProperty1 := "Protocol"
	expectedValue1 := "tcp"
	expectedProperty2 := "Port"
	expectedValue2 := "1883"

	config, err := loadConfigFromFile("", "./test")

	if err != nil {
		t.Errorf("Fail to load config from file, %v", err)
	} else if val, ok := config.Driver[expectedProperty1]; ok != true || val != expectedValue1 {
		t.Errorf("Unexpected test result, '%s' should be exist and value shoud be '%s'", expectedProperty1, expectedValue1)
	} else if val, ok := config.Driver[expectedProperty2]; ok != true || val != expectedValue2 {
		t.Errorf("Unexpected test result, '%s' should be exist and value shoud be '%s'", expectedProperty2, expectedValue2)
	}
}
