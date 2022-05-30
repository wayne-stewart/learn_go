// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package winsvc

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

var log debug.Log

type Service interface {
	Start(elog debug.Log)
	Stop()
}

type ServiceManager struct {
	Name    string
	Desc    string
	Service Service
}

func (mgr *ServiceManager) Run() {

	running_as_service, err := svc.IsWindowsService()
	if err != nil {
		fmt.Printf("failed to determine if we are running in user interactive: %v\n", err)
		os.Exit(1)
	}

	// determine the correct runner based on running in interactive mode or not
	run := svc.Run
	if !running_as_service {
		run = debug.Run
	}

	// determine the correct logger based on running in interactive mode or not
	if running_as_service {
		log, err = eventlog.Open(mgr.Name)
		if err != nil {
			return
		}
		log.Info(1, fmt.Sprintf("%s: starting service", mgr.Name))

	} else {
		log = debug.New(mgr.Name)
		log.Info(1, fmt.Sprintf("%s: starting", mgr.Name))
	}
	defer log.Close()

	err = run(mgr.Name, mgr)
	if err != nil {
		log.Error(1, fmt.Sprintf("%s: service failed: %v", mgr.Name, err))
		return
	}
	log.Info(1, fmt.Sprintf("%s: service stopped", mgr.Name))
}

func (mgr *ServiceManager) Command(cmd string) {
	var err error
	switch cmd {
	case "install":
		err = installService(mgr.Name, mgr.Desc)
	case "remove":
		err = removeService(mgr.Name)
	case "start":
		err = startService(mgr.Name)
	case "stop":
		err = controlService(mgr.Name, svc.Stop, svc.Stopped)
	case "pause":
		err = controlService(mgr.Name, svc.Pause, svc.Paused)
	case "continue":
		err = controlService(mgr.Name, svc.Continue, svc.Running)
	default:
		usage(fmt.Sprintf("invalid command %s", cmd))
	}
	if err != nil {
		fmt.Printf("failed to %s %s: %v\n", cmd, mgr.Name, err)
	}
}

func (mgr *ServiceManager) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	go mgr.Service.Start(log)
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			mgr.Service.Stop()
			break loop
		default:
			log.Error(1, fmt.Sprintf("%s: unexpected control request #%d", mgr.Name, c))
		}
	}

	changes <- svc.Status{State: svc.StopPending}
	return
}

func usage(errmsg string) {
	fmt.Fprintf(os.Stderr,
		"%s\n\n"+
			"usage: %s <command>\n"+
			"       where <command> is one of\n"+
			"       install, remove, start, stop.\n",
		errmsg, os.Args[0])
	os.Exit(2)
}
