// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"
	"github.com/otfabric/go-ieee20305/csipaus"
	"github.com/spf13/cobra"

	"github.com/otfabric/ieee20305ctl/internal/conn"
	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/runner"
	"github.com/otfabric/ieee20305ctl/internal/types"
)

type reportConfig struct {
	Common
	File        string `flag:"file" desc:"File of the MirrorUsagePoint to post (XML), with its MirrorMeterReadings"`
	Power       int64  `flag:"power" desc:"Without --file: post the real power of the device, in watts"`
	Description string `flag:"description" desc:"Without --file: description of the MirrorUsagePoint"`
}

func newReportCmd() *cobra.Command {
	c := reportConfig{Common: newCommon(), Description: "ieee20305ctl"}
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Report metering: post a MirrorUsagePoint with its readings",
		Long: `Post a MirrorUsagePoint to the server's MirrorUsagePointList: the mirror of the device's
metering, which the server makes a UsagePoint with MeterReadings. Give the document as an
XML file, or --power for a mirror with one reading of real power (W), whose mRIDs derive
from the device's LFDI, so that posting again updates the same mirror.`,
		Example: `  ieee20305ctl client report --power 4200
  ieee20305ctl client report --file mup.xml`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			power := cmd.Flags().Changed("power")
			return once(cmd, &c.Common, func() error {
				if (c.File == "") == !power {
					return errs.InvalidInput(errs.CodeInvalidFlagCombination, "give --file or --power", nil)
				}
				return nil
			}, true, func(ctx context.Context, d *conn.Device) (any, error) {
				dcap, err := d.Client.DeviceCapability(ctx)
				if err != nil {
					return nil, err
				}
				if dcap.MirrorUsagePointListLink == nil {
					return nil, errs.New(errs.KindProtocol, errs.CodeUnknownResource, "the server offers no MirrorUsagePointList", nil)
				}
				var doc any
				if c.File != "" {
					if doc, err = readDocument(d, c.File); err != nil {
						return nil, err
					}
				} else {
					doc = powerMirror(d, c.Description, c.Power)
				}
				loc, err := d.Client.Post(ctx, dcap.MirrorUsagePointListLink.Href, doc)
				if err != nil {
					return nil, err
				}
				detail := ""
				if power {
					detail = fmt.Sprintf("real power %d W", c.Power)
				}
				return &types.Posted{Action: "report", Target: dcap.MirrorUsagePointListLink.Href, Location: loc, Detail: detail}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

// powerMirror is a MirrorUsagePoint of the device with one reading of its
// real power: instantaneous, forward, in watts.
func powerMirror(d *conn.Device, description string, watts int64) *ieee20305.MirrorUsagePoint {
	lfdi := d.Identity.LFDI
	desc := ieee20305.String32(description)
	mup := &ieee20305.MirrorUsagePoint{DeviceLFDI: ieee20305.HexBinary160(lfdi[:])}
	mup.MRID = append(ieee20305.MRIDType{}, lfdi[:16]...)
	mup.Description = &desc
	mup.RoleFlags = ieee20305.RoleFlagsType{0x00, 0x49} // isMirror, isDER, isSubmeter
	mup.ServiceCategoryKind = 0                         // electricity
	mup.Status = 1                                      // on
	// Instantaneous, electricity, average; forward, power, no phase, watts.
	acc, com, dq := ieee20305.AccumulationBehaviourType(12), ieee20305.CommodityType(1), ieee20305.DataQualifierType(2)
	flow, kind, phase, uom := ieee20305.FlowDirectionType(1), ieee20305.KindType(37), ieee20305.PhaseCode(0), ieee20305.UomType(38)
	mult := ieee20305.PowerOfTenMultiplierType(0)
	for watts > 1<<46 || watts < -(1<<46) {
		watts /= 10
		mult++
	}
	rt := &ieee20305.ReadingType{AccumulationBehaviour: &acc, Commodity: &com, DataQualifier: &dq, FlowDirection: &flow,
		Kind: &kind, Phase: &phase, PowerOfTenMultiplier: &mult, Uom: &uom}
	now := ieee20305.TimeType(time.Now().Unix())
	value := ieee20305.Int48(watts)
	r := &ieee20305.Reading{}
	r.Value = &value
	r.TimePeriod = &ieee20305.DateTimeInterval{Start: now}
	rdesc := ieee20305.String32("Real power")
	mmr := ieee20305.MirrorMeterReading{ReadingType: rt, Reading: r, LastUpdateTime: &now}
	mmr.MRID = append(ieee20305.MRIDType{}, lfdi[:15]...)
	mmr.MRID = append(mmr.MRID, 0x01)
	mmr.Description = &rdesc
	mup.MirrorMeterReading = []ieee20305.MirrorMeterReading{mmr}
	return mup
}

type logEventConfig struct {
	Common
	Code        uint8  `flag:"code" desc:"logEventCode: the severity (0 informational, 1 warning, 2 error, ...)"`
	ID          uint16 `flag:"id" desc:"logEventID: the event, in the numbering of --pen"`
	PEN         uint32 `flag:"pen" desc:"logEventPEN: the IANA Private Enterprise Number that numbers the event (0: IEEE 2030.5)"`
	FunctionSet uint8  `flag:"function-set" desc:"functionSet the event is about"`
	ProfileID   uint8  `flag:"profile-id" desc:"profileID of the function set"`
	Details     string `flag:"details" desc:"details, at most 32 characters"`
}

func newLogEventCmd() *cobra.Command {
	c := logEventConfig{Common: newCommon()}
	cmd := &cobra.Command{
		Use:     "log-event",
		Short:   "Post a LogEvent of the device",
		Long:    `Post a LogEvent to the LogEventList of the device's EndDevice: an event the device logs for the server.`,
		Example: `  ieee20305ctl client log-event --code 1 --id 7 --details "inverter over temperature"`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return once(cmd, &c.Common, func() error {
				if len(c.Details) > 32 {
					return errs.InvalidInput(errs.CodeInvalidInput, "--details: at most 32 characters", nil)
				}
				return nil
			}, true, func(ctx context.Context, d *conn.Device) (any, error) {
				_, ed, err := endDevice(ctx, d)
				if err != nil {
					return nil, err
				}
				if ed.LogEventListLink == nil {
					return nil, errs.New(errs.KindProtocol, errs.CodeUnknownResource, "the EndDevice has no LogEventListLink", nil)
				}
				ev := &ieee20305.LogEvent{CreatedDateTime: ieee20305.TimeType(time.Now().Unix()), LogEventCode: ieee20305.UInt8(c.Code),
					LogEventID: ieee20305.UInt16(c.ID), LogEventPEN: ieee20305.PENType(c.PEN), FunctionSet: ieee20305.UInt8(c.FunctionSet),
					ProfileID: ieee20305.UInt8(c.ProfileID)}
				if c.Details != "" {
					det := ieee20305.String32(c.Details)
					ev.Details = &det
				}
				loc, err := d.Client.Post(ctx, ed.LogEventListLink.Href, ev)
				if err != nil {
					return nil, err
				}
				return &types.Posted{Action: "log-event", Target: ed.LogEventListLink.Href, Location: loc,
					Detail: fmt.Sprintf("code %d, id %d", c.Code, c.ID)}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

type flowConfig struct {
	Common
	Energy            int64         `flag:"energy" desc:"Energy requested, in Wh (negative: to the grid)"`
	Power             int64         `flag:"power" desc:"Power requested, in W"`
	Start             time.Duration `flag:"start" desc:"Start of the interval, from now"`
	Duration          time.Duration `flag:"duration" desc:"Length of the interval in which the energy is wanted"`
	DurationRequested time.Duration `flag:"duration-requested" desc:"How long the device needs to draw it, within the interval (0: unset)"`
}

func newFlowReservationCmd() *cobra.Command {
	c := flowConfig{Common: newCommon(), Duration: 2 * time.Hour}
	cmd := &cobra.Command{
		Use:   "flow-reservation",
		Short: "Ask the server for a flow reservation (EV charging, storage)",
		Long: `Post a FlowReservationRequest to the device's FlowReservationRequestList: the energy and
power the device wants within an interval, for instance to charge a vehicle. The server
answers with a FlowReservationResponse event, which client run follows.`,
		Example: `  ieee20305ctl client flow-reservation --energy 20000 --power 7000 --start 1h --duration 4h`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return once(cmd, &c.Common, func() error {
				if c.Duration <= 0 || c.Start < 0 {
					return errs.InvalidInput(errs.CodeInvalidInput, "--duration must be positive, and --start not negative", nil)
				}
				return nil
			}, true, func(ctx context.Context, d *conn.Device) (any, error) {
				_, ed, err := endDevice(ctx, d)
				if err != nil {
					return nil, err
				}
				if ed.FlowReservationRequestListLink == nil {
					return nil, errs.New(errs.KindProtocol, errs.CodeUnknownResource, "the EndDevice has no FlowReservationRequestListLink", nil)
				}
				now := time.Now()
				energy, emult := scale48(c.Energy)
				power, pmult, err := scale16(c.Power)
				if err != nil {
					return nil, errs.InvalidInput(errs.CodeInvalidInput, "--power: "+err.Error(), err)
				}
				req := &ieee20305.FlowReservationRequest{CreationTime: ieee20305.TimeType(now.Unix()),
					EnergyRequested:   ieee20305.SignedRealEnergy{Value: ieee20305.Int48(energy), Multiplier: ieee20305.PowerOfTenMultiplierType(emult)},
					PowerRequested:    ieee20305.ActivePower{Value: ieee20305.Int16(power), Multiplier: ieee20305.PowerOfTenMultiplierType(pmult)},
					IntervalRequested: ieee20305.DateTimeInterval{Start: ieee20305.TimeType(now.Add(c.Start).Unix()), Duration: ieee20305.UInt32(c.Duration / time.Second)}}
				req.MRID = append(ieee20305.MRIDType{}, d.Identity.LFDI[:12]...)
				req.MRID = append(req.MRID, byte(now.Unix()>>24), byte(now.Unix()>>16), byte(now.Unix()>>8), byte(now.Unix()))
				if c.DurationRequested > 0 {
					dr := ieee20305.UInt16(c.DurationRequested / time.Second)
					req.DurationRequested = &dr
				}
				loc, err := d.Client.Post(ctx, ed.FlowReservationRequestListLink.Href, req)
				if err != nil {
					return nil, err
				}
				return &types.Posted{Action: "flow-reservation", Target: ed.FlowReservationRequestListLink.Href, Location: loc,
					Detail: fmt.Sprintf("%d Wh at %d W from %s for %s", c.Energy, c.Power, now.Add(c.Start).UTC().Format(time.RFC3339), c.Duration)}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

func scale48(v int64) (int64, int64) {
	m := int64(0)
	for v > 1<<46 || v < -(1<<46) {
		v /= 10
		m++
	}
	return v, m
}

func scale16(v int64) (int64, int64, error) {
	m := int64(0)
	for v > 32767 || v < -32768 {
		if v%10 != 0 {
			return 0, 0, fmt.Errorf("%d W does not fit a value with a power-of-ten multiplier", v)
		}
		v /= 10
		m++
	}
	return v, m, nil
}

func newPricingCmd() *cobra.Command {
	c := newCommon()
	cmd := &cobra.Command{
		Use:   "pricing",
		Short: "List the tariffs the server offers the device, with their prices",
		Long: `List the TariffProfiles of the device's FunctionSetAssignments (or of the DeviceCapability),
with their RateComponents and TimeTariffIntervals, and the price of the first
ConsumptionTariffInterval of each, in the currency and multiplier of the tariff.`,
		Example: `  ieee20305ctl client pricing --format table`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return once(cmd, &c, nil, false, func(ctx context.Context, d *conn.Device) (any, error) {
				return pricing(ctx, d)
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

func pricing(ctx context.Context, d *conn.Device) (*types.Pricing, error) {
	dcap, ed, err := endDevice(ctx, d)
	if err != nil {
		return nil, err
	}
	assignments, err := fsas(ctx, d, ed)
	if err != nil {
		return nil, err
	}
	var lists []string
	for _, f := range assignments {
		if f.TariffProfileListLink != nil {
			lists = append(lists, f.TariffProfileListLink.Href)
		}
	}
	if len(lists) == 0 && dcap.TariffProfileListLink != nil {
		lists = append(lists, dcap.TariffProfileListLink.Href)
	}
	res := &types.Pricing{Tariffs: []types.Tariff{}}
	for _, l := range lists {
		err := d.Client.Iterate(ctx, l, 50, func(it any) error {
			var tp *ieee20305.TariffProfile
			switch x := it.(type) {
			case *ieee20305.TariffProfile:
				tp = x
			case *csipaus.TariffProfile:
				tp = &x.TariffProfile
			default:
				return nil
			}
			t := types.Tariff{Href: tp.GetHref(), Description: str(tp.Description), Primacy: uint8(tp.Primacy), RateComponents: []types.RateComponent{}}
			if tp.Currency != nil {
				cur := uint16(*tp.Currency)
				t.Currency = &cur
			}
			if tp.PricePowerOfTenMultiplier != nil {
				m := int8(*tp.PricePowerOfTenMultiplier)
				t.PriceMultiplier = &m
			}
			if tp.RateCode != nil {
				t.RateCode = string(*tp.RateCode)
			}
			if tp.RateComponentListLink != nil {
				if err := d.Client.Iterate(ctx, tp.RateComponentListLink.Href, 50, func(it any) error {
					rc, ok := it.(*ieee20305.RateComponent)
					if !ok {
						return nil
					}
					comp, err := rateComponent(ctx, d, rc)
					if err != nil {
						return err
					}
					t.RateComponents = append(t.RateComponents, comp)
					return nil
				}); err != nil {
					return err
				}
			}
			res.Tariffs = append(res.Tariffs, t)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func rateComponent(ctx context.Context, d *conn.Device, rc *ieee20305.RateComponent) (types.RateComponent, error) {
	comp := types.RateComponent{Href: rc.GetHref(), Description: str(rc.Description), Intervals: []types.Interval{}}
	if rc.TimeTariffIntervalListLink.Href == "" {
		return comp, nil
	}
	err := d.Client.Iterate(ctx, rc.TimeTariffIntervalListLink.Href, 50, func(it any) error {
		var tti *ieee20305.TimeTariffInterval
		switch x := it.(type) {
		case *ieee20305.TimeTariffInterval:
			tti = x
		case *csipaus.TimeTariffInterval:
			tti = &x.TimeTariffInterval
		default:
			return nil
		}
		iv := types.Interval{Href: tti.GetHref(), Description: str(tti.Description), Start: unix(tti.Interval.Start),
			DurationS: uint32(tti.Interval.Duration), Status: statusText(&tti.EventStatus), TOUTier: uint8(tti.TouTier)}
		if tti.ConsumptionTariffIntervalListLink != nil {
			var ctis ieee20305.ConsumptionTariffIntervalList
			if err := d.Client.GetList(ctx, tti.ConsumptionTariffIntervalListLink.Href, listOne, &ctis); err != nil {
				return err
			}
			if len(ctis.ConsumptionTariffInterval) > 0 && ctis.ConsumptionTariffInterval[0].Price != nil {
				p := int64(*ctis.ConsumptionTariffInterval[0].Price)
				iv.Price = &p
			}
		}
		comp.Intervals = append(comp.Intervals, iv)
		return nil
	})
	return comp, err
}
