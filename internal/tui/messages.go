package tui

import (
	"time"

	"github.com/deveshpharswan/stackup/internal/doctor"
)

type ServiceInfo struct {
	Name   string
	State  string
	Health string
	Ports  string
	Tier   int
	Uptime time.Duration
}

type ServiceUpdateMsg struct {
	Services []ServiceInfo
	Err      error
}

type LogLineMsg struct {
	Line string
}

type LogErrMsg struct {
	Err error
}

type DoctorResultMsg struct {
	Findings []doctor.Finding
}

type DoctorRunningMsg struct{}

type ToastMsg struct {
	Text string
}

type ToastExpiredMsg struct{}

type TickMsg time.Time

type ConfirmAction int

const (
	ConfirmRestart ConfirmAction = iota
	ConfirmDelete
)

type ConfirmRequestMsg struct {
	Action  ConfirmAction
	Service string
}

type ConfirmYesMsg struct {
	Action  ConfirmAction
	Service string
}

type ActionResultMsg struct {
	Text string
	Err  error
}

type shellRequestMsg struct {
	Service string
}
