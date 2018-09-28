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

	"github.com/edgexfoundry/edgex-go/pkg/models"
)

func callbackHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	dec := json.NewDecoder(req.Body)
	cbAlert := models.CallbackAlert{}

	err := dec.Decode(&cbAlert)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		svc.lc.Error(fmt.Sprintf("Invalid callback request: %v", err))
		return
	}

	if (cbAlert.Id == "") || (cbAlert.ActionType == "") {
		http.Error(w, "Missing parameters", http.StatusBadRequest)
		svc.lc.Error(fmt.Sprintf("Missing callback parameters"))
		return
	}

	if (cbAlert.ActionType == models.DEVICE) && (req.Method == http.MethodPost) {
		err = dc.AddById(cbAlert.Id)
		if err == nil {
			svc.lc.Info(fmt.Sprintf("Added device %s", cbAlert.Id))
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			svc.lc.Error(fmt.Sprintf("Couldn't add device %s: %v", cbAlert.Id, err.Error()))
			return
		}
	} else if (cbAlert.ActionType == models.DEVICE) && (req.Method == http.MethodPut) {
		err = dc.UpdateById(cbAlert.Id)
		if err == nil {
			svc.lc.Info(fmt.Sprintf("Updated device %s", cbAlert.Id))
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			svc.lc.Error(fmt.Sprintf("Couldn't update device %s: %v", cbAlert.Id, err.Error()))
			return
		}
	} else if (cbAlert.ActionType == models.DEVICE) && (req.Method == http.MethodDelete) {
		err = dc.RemoveById(cbAlert.Id)
		if err == nil {
			svc.lc.Info(fmt.Sprintf("Removed device %s", cbAlert.Id))
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			svc.lc.Error(fmt.Sprintf("Couldn't remove device %s: %v", cbAlert.Id, err.Error()))
			return
		}
	} else if (cbAlert.ActionType == models.ADDRESSABLE) && (req.Method == http.MethodPut) {
		err = dc.UpdateAddressableById(cbAlert.Id)
		if err == nil {
			svc.lc.Info(fmt.Sprintf("Updated addressable %s", cbAlert.Id))
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			svc.lc.Error(fmt.Sprintf("Couldn't addressable device %s: %v", cbAlert.Id, err.Error()))
			return
		}
	} else {
		svc.lc.Error(fmt.Sprintf("Invalid device method and/or action type: %s - %s", req.Method, cbAlert.ActionType))
		http.Error(w, "Invalid device method and/or action type", http.StatusBadRequest)
		return
	}

	io.WriteString(w, "OK")
}

func initUpdate() {
	svc.r.HandleFunc("/callback", callbackHandler)
}
