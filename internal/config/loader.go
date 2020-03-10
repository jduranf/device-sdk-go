// -*- mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
// Copyright (c) 2019 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/edgexfoundry/device-sdk-go/internal/common"
	"github.com/pelletier/go-toml"
)

// LoadConfig loads the local configuration file based upon the
// specified parameters and returns a pointer to the global Config
// struct which holds all of the local configuration settings for
// the DS. The profile and confDir are used to locate the local TOML
// config file.
func LoadConfig(profile string, confDir string) (*common.Config, error) {
	fmt.Fprintf(os.Stdout, "Init: profile: %s confDir: %s\n", profile, confDir)

	return loadConfigFromFile(profile, confDir)
}

func loadConfigFromFile(profile string, confDir string) (config *common.Config, err error) {
	if len(confDir) == 0 {
		confDir = common.ConfigDirectory
	}

	path := path.Join(confDir, common.ConfigFileName)
	absPath, err := filepath.Abs(path)
	if err != nil {
		err = fmt.Errorf("Could not create absolute path to load configuration: %s; %v", path, err.Error())
		return nil, err
	}
	fmt.Fprintln(os.Stdout, fmt.Sprintf("Loading configuration from: %s\n", absPath))

	// As the toml package can panic if TOML is invalid,
	// or elements are found that don't match members of
	// the given struct, use a defered func to recover
	// from the panic and output a useful error.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("could not load configuration file; invalid TOML (%s)", path)
		}
	}()

	config = &common.Config{}
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Could not load configuration file (%s): %v\nBe sure to change to program folder or set working directory.", path, err.Error())
	}

	// Decode the configuration from TOML
	//
	// TODO: invalid input can cause a SIGSEGV fatal error (INVESTIGATE)!!!
	//       - test missing keys, keys with wrong type, ...
	err = toml.Unmarshal(contents, config)
	if err != nil {
		return nil, fmt.Errorf("unable to parse configuration file (%s): %v", path, err.Error())
	}

	return config, nil
}
