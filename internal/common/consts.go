// -*- mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
// Copyright (C) 2018-2019 IOTech Ltd
// Copyright (c) 2019 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"github.com/Circutor/edgex/pkg/clients"
)

const (
	ClientData     = "Data"
	ClientMetadata = "Metadata"
	ClientLogging  = "Logging"

	APIv1Prefix = "/api/v1"
	Colon       = ":"
	HttpScheme  = "http://"
	HttpProto   = "HTTP"

	ConfigDirectory = "./res"
	ConfigFileName  = "configuration.toml"

	APICallbackRoute = APIv1Prefix + "/callback"
	APIPingRoute     = APIv1Prefix + "/ping"

	NameVar      string = "name"
	CommandVar   string = "command"
	GetCmdMethod string = "get"

	CorrelationHeader = clients.CorrelationHeader
)
