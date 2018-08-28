// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
//
// SPDX-License-Identifier: Apache-2.0
//
package device

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/edgexfoundry/edgex-go/pkg/models"
)

func callbackHandler(w http.ResponseWriter, req *http.Request) {
	// use req.Method vs. method

	dec := json.NewDecoder(req.Body)
	cbAlert := models.CallbackAlert{}

	err := dec.Decode(&cbAlert)
	if err != nil {
		// TODO: handle error properly
		fmt.Fprintf(os.Stderr, "service: callbackHandler invalid request: %v\n", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	method := req.Method
	action := cbAlert.ActionType
	id := cbAlert.Id

	//fmt.Fprintf(os.Stderr, "service: callbackHandler action: %v id: %s\n", action, id)
	fmt.Fprintf(os.Stderr, "service: callbackHandler method: %s - action: %v - id: %s\n", method, action, id)

	io.WriteString(w, "OK")
}

func initUpdate(s *Service) {
	s.r.HandleFunc("/callback", callbackHandler)
}
