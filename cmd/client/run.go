// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"
	"github.com/otfabric/go-ieee20305/agent"
	"github.com/otfabric/go-ieee20305/schedule"
	"github.com/spf13/cobra"

	"github.com/otfabric/ieee20305ctl/internal/conn"
	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/format"
	"github.com/otfabric/ieee20305ctl/internal/ieee"
	"github.com/otfabric/ieee20305ctl/internal/runner"
	"github.com/otfabric/ieee20305ctl/internal/types"
)

type runConfig struct {
	Common
	PIN        uint32        `flag:"pin" env:"IEEE20305CTL_PIN" desc:"PIN the device was given out of band; the agent stops when the Registration has another (0: do not check)"`
	PollRate   time.Duration `flag:"poll-rate" desc:"Poll at this rate instead of the server's pollRate"`
	Duration   time.Duration `flag:"duration" desc:"Stop after this long (0: until interrupted)"`
	Power      int64         `flag:"report-power" desc:"Report this real power (W) through a metering mirror at --post-rate (0: no metering)"`
	PostRate   time.Duration `flag:"post-rate" desc:"Rate of the metering reports of --report-power"`
	Listen     string        `flag:"notify-listen" desc:"Serve a notification listener on this address and subscribe to the device's programs (needs --notify-url when the address is not reachable as is)"`
	NotifyURL  string        `flag:"notify-url" desc:"URL the server posts notifications to"`
	Subscribed time.Duration `flag:"subscribed-poll-rate" desc:"Poll rate while subscribed"`
	Polls      bool          `flag:"polls" desc:"Write a record for each poll too, not only for failed ones"`
}

func newRunCmd() *cobra.Command {
	c := runConfig{Common: newCommon(), PostRate: time.Minute, Subscribed: 15 * time.Minute}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a device agent: register, poll, schedule events and answer them",
		Long: `Run the device agent of go-ieee20305 as the device: it registers (checking --pin), polls
its FunctionSetAssignments at the server's pollRate, schedules the events of its DER,
demand response, pricing, messaging, prepayment and flow reservation programs with
primacy and randomization, and posts the Responses they ask for. With --report-power it
mirrors its metering; with --notify-listen it subscribes to its programs and polls less.

One record per thing that happens: started, a DER control (or other event) that is
Scheduled, Active, Completed, Cancelled or Superseded, with the controls it sets; the
default control; each Response posted; failed polls. With --format json the records are
JSON Lines. It stops when interrupted, after --duration, or on an error that waiting does
not fix (a refused certificate, a PIN that does not match), with that error's exit code.`,
		Example: `  ieee20305ctl client run --pin 111115
  ieee20305ctl client run --pin 111115 --report-power 4200 --duration 1h --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.RunStream(cmd, c.Format, func() error {
				if err := c.Validate(); err != nil {
					return err
				}
				if c.Listen != "" && c.NotifyURL == "" && strings.HasPrefix(c.Listen, "0.0.0.0") {
					return errs.InvalidInput(errs.CodeInvalidFlagCombination, "--notify-listen on 0.0.0.0 needs --notify-url, the URL the server reaches", nil)
				}
				return nil
			}, c.run)
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

// lockedWriter serializes the records of the agent's goroutines.
type lockedWriter struct {
	mu  sync.Mutex
	out *format.StreamWriter
	err error
}

func (w *lockedWriter) write(e *types.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err == nil {
		w.err = w.out.Write(e)
	}
}

// agentMetrics turns what the agent reports into records.
type agentMetrics struct {
	w     *lockedWriter
	polls bool
}

func (m agentMetrics) OnPoll(d time.Duration, err error) {
	switch {
	case err != nil:
		m.w.write(&types.Event{Time: now(), Kind: "poll_failed", Message: err.Error()})
	case m.polls:
		m.w.write(&types.Event{Time: now(), Kind: "poll", Message: d.Round(time.Millisecond).String()})
	}
}

func (m agentMetrics) OnEvent(agent.FunctionSet, schedule.Transition) {}

func (m agentMetrics) OnResponse(fs agent.FunctionSet, status uint8, err error) {
	e := &types.Event{Time: now(), Kind: "response", Code: int(status), Status: statusName(status), Resource: fs.String()}
	if err != nil {
		e.Kind, e.Message = "response_failed", err.Error()
	}
	m.w.write(e)
}

var functionSetEvents = map[agent.FunctionSet]string{
	agent.DER: "der_control", agent.DRLC: "load_control", agent.Messaging: "message", agent.Pricing: "price",
	agent.Prepayment: "supply_override", agent.FlowReservation: "flow_reservation",
}

func (c *runConfig) run(ctx context.Context, out *format.StreamWriter) (int, error) {
	d, err := c.Connect()
	if err != nil {
		return 0, err
	}
	if c.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Duration)
		defer cancel()
	}
	w := &lockedWriter{out: out}
	var a *agent.Agent
	var started sync.Once
	transition := func(fs agent.FunctionSet) func(context.Context, schedule.Transition) error {
		return func(_ context.Context, t schedule.Transition) error {
			start, end := t.Event.Start, t.Event.Start.Add(t.Event.Duration)
			program, _, _ := strings.Cut(t.Event.Program, "|")
			e := &types.Event{Time: now(), Kind: functionSetEvents[fs], Status: t.To.String(), MRID: t.Event.MRID,
				Resource: program, Start: &start, End: &end}
			if b, ok := ieee.BaseOf(t.Event.Value); ok {
				e.Settings = ieee.Settings(b)
			}
			w.write(e)
			return nil
		}
	}
	cfg := agent.Config{
		Client: d.Client, Identity: d.Identity, PIN: c.PIN, VerifyPIN: c.PIN != 0, PollRate: c.PollRate,
		OnDERControl: transition(agent.DER), OnEndDeviceControl: transition(agent.DRLC),
		OnTextMessage: transition(agent.Messaging), OnTimeTariffInterval: transition(agent.Pricing),
		OnSupplyInterruptionOverride: transition(agent.Prepayment), OnFlowReservation: transition(agent.FlowReservation),
		OnDefaultDERControl: func(_ context.Context, doc any) error {
			e := &types.Event{Time: now(), Kind: "default_control"}
			if b, ok := ieee.BaseOf(doc); ok {
				e.Settings = ieee.Settings(b)
			}
			if h, ok := doc.(ieee20305.Hrefer); ok {
				e.Href = h.GetHref()
			}
			w.write(e)
			return nil
		},
		OnAssignments: func(_ context.Context, f []*ieee20305.FunctionSetAssignments) error {
			started.Do(func() {
				e := &types.Event{Time: now(), Kind: "started", LFDI: d.Identity.LFDI.String()}
				if a != nil {
					st := a.State()
					if st.EndDevice != nil {
						e.Href = st.EndDevice.GetHref()
					}
					if st.Registration != nil {
						e.Message = fmt.Sprintf("registered with PIN %d", st.Registration.PIN)
					}
				}
				w.write(e)
			})
			w.write(&types.Event{Time: now(), Kind: "assignments", Message: fmt.Sprintf("%d FunctionSetAssignments", len(f))})
			return nil
		},
		Metrics: agentMetrics{w: w, polls: c.Polls},
	}
	if c.Power != 0 {
		cfg.MirrorUsagePoint = powerMirror(d, "ieee20305ctl", c.Power)
		cfg.PostRate = c.PostRate
		cfg.Readings = func(context.Context) ([]ieee20305.MirrorMeterReading, error) {
			return powerMirror(d, "ieee20305ctl", c.Power).MirrorMeterReading, nil
		}
	}
	var stopListener func()
	if c.Listen != "" {
		cfg.NotificationURI = c.NotifyURL
		cfg.SubscribedPollRate = c.Subscribed
	}
	a, err = agent.New(cfg)
	if err != nil {
		return 0, errs.InvalidInput(errs.CodeInvalidConfig, err.Error(), err)
	}
	defer a.Close()
	if c.Listen != "" {
		mux := http.NewServeMux()
		mux.Handle("/notify", a.NotificationHandler())
		local, stop, err := listen(ctx, c.Listen, d.Cert, d, mux)
		if err != nil {
			return 0, err
		}
		stopListener = stop
		defer stopListener()
		w.write(&types.Event{Time: now(), Kind: "listening", Address: local})
	}
	if err := a.Run(ctx); err != nil && ctx.Err() == nil {
		return 0, conn.Classify(err, false)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return 0, errs.Output(errs.CodeOutputWriteFailed, w.err)
	}
	return errs.ExitOK, nil
}
