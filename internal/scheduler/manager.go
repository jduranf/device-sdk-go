// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"fmt"
	"sync"

	"github.com/edgexfoundry/device-sdk-go/internal/cache"
	"github.com/edgexfoundry/device-sdk-go/internal/common"
	"github.com/edgexfoundry/edgex-go/pkg/models"
	"gopkg.in/robfig/cron.v2"
)

var (
	schMgrOnce sync.Once
	cr         *cron.Cron
	entryMap   map[string]cron.EntryID
)

func StartScheduler() {
	schMgrOnce.Do(func() {
		cr = cron.New()
		cr.Start()
		entryMap = make(map[string]cron.EntryID, 0)
		schEvts := cache.ScheduleEvents().All()
		for _, schEvt := range schEvts {
			err := AddScheduleEvent(schEvt)
			if err != nil {
				common.LoggingClient.Error(err.Error())
			}
		}
	})
}

func AddScheduleEvent(schEvt models.ScheduleEvent) error {
	cr.Stop()
	defer cr.Start()

	if _, ok := entryMap[schEvt.Name]; ok {
		return fmt.Errorf("Schedule event %s already exists in scheduler", schEvt.Name)
	}

	sch, ok := cache.Schedules().ForName(schEvt.Schedule)
	if !ok {
		return fmt.Errorf("Schedule %s for schedule event %s cannot be found in cache", schEvt.Schedule, schEvt.Name)
	}
	exec := schEvtExec{schEvt: schEvt, sch: sch}

	spec, err := exec.cronSpec()
	if err != nil {
		return err
	}
	entry, err := cr.AddJob(spec, &exec)
	if err != nil {
		return err
	}
	entryMap[schEvt.Name] = entry
	common.LoggingClient.Info(fmt.Sprintf("Initialized schedule event %s", schEvt.Name))
	return nil
}

func RemoveScheduleEvent(name string) error {
	entry, ok := entryMap[name]
	if !ok {
		return fmt.Errorf("Schedule event %s does not exist in scheduler", name)
	}

	cr.Remove(entry)
	delete(entryMap, name)
	return nil
}

func StopScheduler() {
	cr.Stop()
	common.LoggingClient.Info("Stopped internal scheduler")
}
